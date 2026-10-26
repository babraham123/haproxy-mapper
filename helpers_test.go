package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/maxmind/mmdbwriter"
	"github.com/maxmind/mmdbwriter/mmdbtype"
)

// fixtures is the test Fetcher: response bodies by URL.
type fixtures map[string][]byte

func (f fixtures) Fetch(url string) (io.ReadCloser, error) {
	body, ok := f[url]
	if !ok {
		return nil, fmt.Errorf("no fixture for %s", url)
	}
	return io.NopCloser(bytes.NewReader(body)), nil
}

// recorder is a Receiver that keeps the map lines it's sent.
type recorder struct {
	lines []string
	done  bool
}

func (r *recorder) Subscribed(id string)                  {}
func (r *recorder) Header(id string, header Header) error { return nil }
func (r *recorder) Done(id string) error                  { r.done = true; return nil }
func (r *recorder) Receive(id string, block *Block) error {
	r.lines = append(r.lines, block.net.String()+" "+*block.value)
	return nil
}

func mustBlock(t *testing.T, cidr, value string) *Block {
	t.Helper()
	block, err := BlockCreateWithCidr(&cidr, &value)
	if err != nil {
		t.Fatal(err)
	}
	return block
}

// readLines returns a map or pattern file's lines, without comments.
func readLines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := make([]string, 0)
	for _, line := range strings.Split(string(data), "\n") {
		if line != "" && !strings.HasPrefix(line, "#") {
			lines = append(lines, line)
		}
	}
	return lines
}

func assertLines(t *testing.T, desc string, got, expected []string) {
	t.Helper()
	if len(got) == 0 && len(expected) == 0 {
		return
	}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("%s:\n got      %q\n expected %q", desc, got, expected)
	}
}

// listDir returns the names of dir's entries.
func listDir(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0)
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

type mmdbRecord struct {
	cidr string
	data mmdbtype.Map
}

// buildMMDB returns a MaxMind database of databaseType holding records.
func buildMMDB(t *testing.T, databaseType string, records []mmdbRecord) []byte {
	t.Helper()
	tree, err := mmdbwriter.New(mmdbwriter.Options{
		DatabaseType:            databaseType,
		Description:             map[string]string{"en": "test " + databaseType},
		IncludeReservedNetworks: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		_, network, err := net.ParseCIDR(record.cidr)
		if err != nil {
			t.Fatal(err)
		}
		if err := tree.Insert(network, record.data); err != nil {
			t.Fatal(err)
		}
	}
	var buf bytes.Buffer
	if _, err := tree.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func writeFile(t *testing.T, path string, data []byte) string {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// geoLite2Archive packages db the way MaxMind's download does:
// <edition>_<date>/<edition>.mmdb alongside license files.
func geoLite2Archive(t *testing.T, edition string, db []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	archive := tar.NewWriter(gz)
	dir := edition + "_20261020"
	for _, file := range []struct {
		name string
		data []byte
	}{
		{dir + "/LICENSE.txt", []byte("license")},
		{dir + "/" + edition + ".mmdb", db},
	} {
		if err := archive.WriteHeader(&tar.Header{Name: file.name, Mode: 0o644, Size: int64(len(file.data)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := archive.Write(file.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func geoLite2URLFor(edition, key string) string {
	return fmt.Sprintf(geoLite2URL, edition, key)
}

// runPipeline builds and runs opts, failing the test on any error.
func runPipeline(t *testing.T, opts Options) {
	t.Helper()
	p, err := BuildPipeline(opts)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Run(); err != nil {
		t.Fatal(err)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
}

// readMaps returns the lines of each of names' files in dir.
func readMaps(t *testing.T, dir string, names ...string) map[string][]string {
	t.Helper()
	maps := make(map[string][]string)
	for _, name := range names {
		maps[name] = readLines(t, filepath.Join(dir, name))
	}
	return maps
}
