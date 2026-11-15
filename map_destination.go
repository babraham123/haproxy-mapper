package main

import (
	"bufio"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// pendingFile is written under a temporary name in tmpDir and renamed over
// target on commit. tmpDir must be on target's filesystem so the rename is
// atomic and readers of target see the old file or the new one, never a
// partial one.
type pendingFile struct {
	*bufio.Writer
	fh        *os.File
	target    string
	closed    bool
	committed bool
}

func pendingFileCreate(tmpDir, target string) (*pendingFile, error) {
	fh, err := os.CreateTemp(tmpDir, "."+filepath.Base(target)+".tmp-")
	if err != nil {
		return nil, err
	}
	// CreateTemp makes the file 0600, but HAProxy may run as another user.
	if err := fh.Chmod(0o644); err != nil {
		fh.Close()
		os.Remove(fh.Name())
		return nil, err
	}
	return &pendingFile{Writer: bufio.NewWriter(fh), fh: fh, target: target}, nil
}

// close flushes and closes the temporary file, leaving it under its temporary
// name.
func (p *pendingFile) close() error {
	if p.closed {
		return nil
	}
	p.closed = true
	err := p.Flush()
	if closeErr := p.fh.Close(); err == nil {
		err = closeErr
	}
	return err
}

func (p *pendingFile) commit() error {
	if p.committed {
		return nil
	}
	if err := p.close(); err != nil {
		return err
	}
	if err := os.Rename(p.fh.Name(), p.target); err != nil {
		return err
	}
	p.committed = true
	return nil
}

// discard removes the temporary file unless it was committed.
func (p *pendingFile) discard() error {
	if p.committed {
		return nil
	}
	p.close()
	if err := os.Remove(p.fh.Name()); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// MapDestination writes one HAProxy map file, replacing Filename only once
// the stream is Done.
type MapDestination struct {
	Filename string
	file     *pendingFile
}

func MapDestinationCreate(filename string) (*MapDestination, error) {
	file, err := pendingFileCreate(filepath.Dir(filename), filename)
	if err != nil {
		return nil, err
	}
	return &MapDestination{Filename: filename, file: file}, nil
}

func (m *MapDestination) Subscribed(id string) {}

func (m *MapDestination) Header(id string, header Header) error {
	if len(header.general) > 0 {
		if _, err := m.file.WriteString(header.general); err != nil {
			return err
		}
	}
	if len(header.columns) > 0 {
		if _, err := m.file.WriteString(header.columns); err != nil {
			return err
		}
	}
	return nil
}

func (m *MapDestination) Receive(id string, block *Block) error {
	if m.file.closed {
		return errors.New("Write called on closed Map")
	}

	if _, err := m.file.WriteString(block.net.String()); err != nil {
		return err
	}
	if err := m.file.WriteByte(' '); err != nil {
		return err
	}
	if _, err := m.file.WriteString(*block.value); err != nil {
		return err
	}
	return m.file.WriteByte('\n')
}

func (m *MapDestination) Done(id string) error {
	return m.file.commit()
}

// Close removes the temporary file when the stream never reached Done,
// leaving Filename as it was.
func (m *MapDestination) Close() error {
	return m.file.discard()
}
