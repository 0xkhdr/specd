package cmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// makeTarball returns a gzip-compressed tar containing a single "specd" file.
func makeTarball(t *testing.T, payload string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	body := []byte(payload)
	if err := tw.WriteHeader(&tar.Header{Name: "specd", Mode: 0o755, Size: int64(len(body))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// serveRelease stands up a fake release server. sums maps asset name → contents
// of SHA256SUMS; if checksum is "" the SHA256SUMS endpoint 404s.
func serveRelease(t *testing.T, tarName string, tarball []byte, checksum string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1.0.0/"+tarName, func(w http.ResponseWriter, r *http.Request) {
		w.Write(tarball)
	})
	mux.HandleFunc("/v1.0.0/SHA256SUMS", func(w http.ResponseWriter, r *http.Request) {
		if checksum == "" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		fmt.Fprintf(w, "%s  %s\n", checksum, tarName)
	})
	return httptest.NewServer(mux)
}

func TestDownloadBinary(t *testing.T) {
	tarName := fmt.Sprintf("specd_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	tarball := makeTarball(t, "#!/bin/true\n")
	good := sha256Hex(tarball)

	t.Run("verified_checksum_installs_binary", func(t *testing.T) {
		srv := serveRelease(t, tarName, tarball, good)
		defer srv.Close()
		old := releaseBaseURL
		releaseBaseURL = srv.URL
		defer func() { releaseBaseURL = old }()

		dest := filepath.Join(t.TempDir(), "specd")
		if err := downloadBinary("v1.0.0", dest); err != nil {
			t.Fatalf("downloadBinary: %v", err)
		}
		if _, err := os.Stat(dest); err != nil {
			t.Errorf("dest not written: %v", err)
		}
	})

	t.Run("checksum_mismatch_aborts_and_leaves_no_new_file", func(t *testing.T) {
		srv := serveRelease(t, tarName, tarball, strings.Repeat("0", 64))
		defer srv.Close()
		old := releaseBaseURL
		releaseBaseURL = srv.URL
		defer func() { releaseBaseURL = old }()

		dir := t.TempDir()
		dest := filepath.Join(dir, "specd")
		err := downloadBinary("v1.0.0", dest)
		if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
			t.Fatalf("want checksum mismatch error, got %v", err)
		}
		if _, statErr := os.Stat(dest); statErr == nil {
			t.Error("dest should not exist on mismatch")
		}
		if _, statErr := os.Stat(dest + ".new"); statErr == nil {
			t.Error(".new file should not be left behind")
		}
		entries, _ := os.ReadDir(dir)
		if len(entries) != 0 {
			t.Errorf("temp leftovers: %d entries remain", len(entries))
		}
	})

	t.Run("missing_checksums_fails_closed", func(t *testing.T) {
		srv := serveRelease(t, tarName, tarball, "")
		defer srv.Close()
		old := releaseBaseURL
		releaseBaseURL = srv.URL
		defer func() { releaseBaseURL = old }()

		dest := filepath.Join(t.TempDir(), "specd")
		if err := downloadBinary("v1.0.0", dest); err == nil {
			t.Fatal("want error when SHA256SUMS missing, got nil")
		}
		if _, err := os.Stat(dest); err == nil {
			t.Error("dest should not exist when checksums missing")
		}
	})
}
