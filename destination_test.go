package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMapDestinationReplacesOnDone(t *testing.T) {
	dir := t.TempDir()
	target := writeFile(t, filepath.Join(dir, "ip_to_test"), []byte("10.0.0.0/8 OLD\n"))

	dest, err := MapDestinationCreate(target)
	if err != nil {
		t.Fatal(err)
	}
	if err := dest.Header("test", Header{general: "# test map\n"}); err != nil {
		t.Fatal(err)
	}
	if err := dest.Receive("test", mustBlock(t, "192.0.2.0/24", "NEW")); err != nil {
		t.Fatal(err)
	}
	assertLines(t, "before Done", readLines(t, target), []string{"10.0.0.0/8 OLD"})

	if err := dest.Done("test"); err != nil {
		t.Fatal(err)
	}
	if err := dest.Close(); err != nil {
		t.Fatal(err)
	}
	assertLines(t, "after Done", readLines(t, target), []string{"192.0.2.0/24 NEW"})
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o644 {
		t.Errorf("mode %o, expected 644", mode)
	}
	assertLines(t, "files", listDir(t, dir), []string{"ip_to_test"})
}

func TestFailedRunKeepsLiveMaps(t *testing.T) {
	outdir := t.TempDir()
	writeFile(t, filepath.Join(outdir, "ip_to_aws"), []byte("10.0.0.0/8 OLD\n"))

	p, err := BuildPipeline(Options{Outdir: outdir, Maps: selecting("aws"), IPv6: true, Fetcher: fixtures{}})
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Run(); err == nil {
		t.Fatal("Run succeeded without a feed to fetch")
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	assertLines(t, "ip_to_aws", readLines(t, filepath.Join(outdir, "ip_to_aws")), []string{"10.0.0.0/8 OLD"})
	assertLines(t, "files", listDir(t, outdir), []string{"ip_to_aws"})
}

func TestSplitMapDestinationReplacesBeforeRemoving(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "AU.map"), []byte("10.0.0.0/8\n"))
	// A non-empty directory can't be removed, so Done fails while removing
	// the maps dropped from the set.
	if err := os.MkdirAll(filepath.Join(dir, "XX.map", "keep"), 0o755); err != nil {
		t.Fatal(err)
	}

	dest, err := SplitMapDestinationCreate(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := dest.Receive("test", mustBlock(t, "1.0.0.0/24", "AU")); err != nil {
		t.Fatal(err)
	}
	if err := dest.Done("test"); err == nil {
		t.Fatal("Done removed a non-empty directory")
	}
	if err := dest.Close(); err != nil {
		t.Fatal(err)
	}
	assertLines(t, "AU.map", readLines(t, filepath.Join(dir, "AU.map")), []string{"1.0.0.0/24"})
	assertLines(t, "files", listDir(t, dir), []string{"AU.map", "XX.map"})
}
