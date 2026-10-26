package main

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
)

// Values become file names, so anything beyond plain codes is skipped.
var splitValuePattern = regexp.MustCompile(`^[A-Za-z0-9]+$`)

// SplitMapDestination writes one HAProxy pattern file per value, <value>.map
// with a CIDR per line, into dir. Files are built in a staging directory and
// replace dir's existing *.map files only once the stream is Done, so readers
// never see a partial set.
type SplitMapDestination struct {
	dir     string
	staging string
	files   map[string]*os.File
	bufs    map[string]*bufio.Writer
}

func SplitMapDestinationCreate(dir string) (*SplitMapDestination, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	staging, err := os.MkdirTemp(dir, ".staging-")
	if err != nil {
		return nil, err
	}
	return &SplitMapDestination{
		dir:     dir,
		staging: staging,
		files:   make(map[string]*os.File),
		bufs:    make(map[string]*bufio.Writer),
	}, nil
}

func (s *SplitMapDestination) Subscribed(id string) {}

func (s *SplitMapDestination) Header(id string, header Header) error {
	return nil
}

func (s *SplitMapDestination) Receive(id string, block *Block) error {
	value := *block.value
	if !splitValuePattern.MatchString(value) {
		return nil
	}
	buf, ok := s.bufs[value]
	if !ok {
		fh, err := os.Create(filepath.Join(s.staging, value+".map"))
		if err != nil {
			return err
		}
		s.files[value] = fh
		buf = bufio.NewWriter(fh)
		s.bufs[value] = buf
	}
	if _, err := buf.WriteString(block.net.String()); err != nil {
		return err
	}
	return buf.WriteByte('\n')
}

func (s *SplitMapDestination) Done(id string) error {
	for value, fh := range s.files {
		if err := s.bufs[value].Flush(); err != nil {
			return err
		}
		if err := fh.Close(); err != nil {
			return err
		}
	}

	old, err := filepath.Glob(filepath.Join(s.dir, "*.map"))
	if err != nil {
		return err
	}
	for _, path := range old {
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	for value := range s.files {
		name := value + ".map"
		if err := os.Rename(filepath.Join(s.staging, name), filepath.Join(s.dir, name)); err != nil {
			return err
		}
	}
	return s.Close()
}

// Close removes the staging directory, discarding anything not yet moved into
// place.
func (s *SplitMapDestination) Close() error {
	for _, fh := range s.files {
		fh.Close()
	}
	return os.RemoveAll(s.staging)
}
