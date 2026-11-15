package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Values become file names, so anything beyond plain codes is skipped.
var splitValuePattern = regexp.MustCompile(`^[A-Za-z0-9]+$`)

const stagingPrefix = ".staging-"

// SplitMapDestination writes one HAProxy pattern file per value, <value>.map
// with a CIDR per line, into dir. Files are built in a staging directory and
// moved into place once the stream is Done. Each file is renamed over its
// predecessor, so a reader sees the old or the new version of a map, never a
// partial one, and a map in both the old and new sets never goes missing. The
// set as a whole isn't swapped atomically: while Done runs, a reader can see
// a mix of old and new maps, and maps dropped from the set are deleted last.
type SplitMapDestination struct {
	dir     string
	staging string
	files   map[string]*pendingFile
}

func SplitMapDestinationCreate(dir string) (*SplitMapDestination, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	// A killed run leaves its staging directory behind.
	stale, err := filepath.Glob(filepath.Join(dir, stagingPrefix+"*"))
	if err != nil {
		return nil, err
	}
	for _, path := range stale {
		if err := os.RemoveAll(path); err != nil {
			return nil, err
		}
	}
	staging, err := os.MkdirTemp(dir, stagingPrefix)
	if err != nil {
		return nil, err
	}
	return &SplitMapDestination{
		dir:     dir,
		staging: staging,
		files:   make(map[string]*pendingFile),
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
	file, ok := s.files[value]
	if !ok {
		var err error
		if file, err = pendingFileCreate(s.staging, filepath.Join(s.dir, value+".map")); err != nil {
			return err
		}
		s.files[value] = file
	}
	if _, err := file.WriteString(block.net.String()); err != nil {
		return err
	}
	return file.WriteByte('\n')
}

func (s *SplitMapDestination) Done(id string) error {
	// Write every file out before replacing any, so a write error leaves dir
	// untouched.
	for _, file := range s.files {
		if err := file.close(); err != nil {
			return err
		}
	}
	for _, file := range s.files {
		if err := file.commit(); err != nil {
			return err
		}
	}

	old, err := filepath.Glob(filepath.Join(s.dir, "*.map"))
	if err != nil {
		return err
	}
	for _, path := range old {
		if _, kept := s.files[strings.TrimSuffix(filepath.Base(path), ".map")]; kept {
			continue
		}
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	return s.Close()
}

// Close removes the staging directory, discarding anything not yet moved into
// place.
func (s *SplitMapDestination) Close() error {
	for _, file := range s.files {
		file.close()
	}
	return os.RemoveAll(s.staging)
}
