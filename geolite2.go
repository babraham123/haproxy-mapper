package main

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const geoLite2URL = "https://download.maxmind.com/app/geoip_download?edition_id=%s&license_key=%s&suffix=tar.gz"

// DownloadGeoLite2 fetches a free MaxMind GeoLite2 edition, e.g.
// GeoLite2-Country, and extracts its database into dir, returning the
// database's path.
func DownloadGeoLite2(f Fetcher, edition, licenseKey, dir string) (string, error) {
	escapedKey := url.QueryEscape(licenseKey)
	// Fetch errors can carry the URL, which carries the license key.
	redact := func(err error) error {
		msg := err.Error()
		if licenseKey != "" {
			msg = strings.ReplaceAll(msg, escapedKey, "REDACTED")
			msg = strings.ReplaceAll(msg, licenseKey, "REDACTED")
		}
		return fmt.Errorf("downloading %s: %s", edition, msg)
	}

	body, err := f.Fetch(fmt.Sprintf(geoLite2URL, url.QueryEscape(edition), escapedKey))
	if err != nil {
		return "", redact(err)
	}
	defer body.Close()

	gz, err := gzip.NewReader(body)
	if err != nil {
		return "", redact(err)
	}
	archive := tar.NewReader(gz)
	for {
		hdr, err := archive.Next()
		if errors.Is(err, io.EOF) {
			return "", fmt.Errorf("downloading %s: %s.mmdb not found in archive", edition, edition)
		} else if err != nil {
			return "", redact(err)
		}
		if hdr.Typeflag != tar.TypeReg || path.Base(hdr.Name) != edition+".mmdb" {
			continue
		}

		dbPath := filepath.Join(dir, edition+".mmdb")
		out, err := os.Create(dbPath)
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(out, archive); err != nil {
			out.Close()
			return "", redact(err)
		}
		return dbPath, out.Close()
	}
}
