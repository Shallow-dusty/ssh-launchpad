package launchpad

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestDownloadVerifiedResumesAndChecksSHA256(t *testing.T) {
	payload := []byte(strings.Repeat("launchpad", 1024))
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := 0
		if value := r.Header.Get("Range"); strings.HasPrefix(value, "bytes=") {
			start, _ = strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(value, "bytes="), "-"))
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(payload)-1, len(payload)))
			w.WriteHeader(http.StatusPartialContent)
		}
		_, _ = w.Write(payload[start:])
	}))
	defer server.Close()
	sum := sha256.Sum256(payload)
	output := filepath.Join(t.TempDir(), "asset")
	if err := os.WriteFile(output+".part", payload[:100], 0o644); err != nil {
		t.Fatal(err)
	}
	old := http.DefaultTransport
	http.DefaultTransport = server.Client().Transport
	defer func() { http.DefaultTransport = old }()
	err := DownloadVerified(context.Background(), DownloadRequest{
		URL:       server.URL,
		Output:    output,
		SHA256:    hex.EncodeToString(sum[:]),
		Retries:   1,
		AllowHTTP: true,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(output)
	if string(got) != string(payload) {
		t.Fatal("downloaded payload differs")
	}
}

func TestDownloadRejectsMissingChecksum(t *testing.T) {
	err := DownloadVerified(context.Background(), DownloadRequest{URL: "https://example.com/a", Output: filepath.Join(t.TempDir(), "a")}, nil)
	if err == nil {
		t.Fatal("expected checksum requirement")
	}
}

func TestDownloadFallsBackToNextVerifiedSource(t *testing.T) {
	payload := []byte("verified fallback")
	failing := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "temporary failure", http.StatusServiceUnavailable)
	}))
	defer failing.Close()
	working := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer working.Close()
	sum := sha256.Sum256(payload)
	old := http.DefaultTransport
	http.DefaultTransport = working.Client().Transport
	defer func() { http.DefaultTransport = old }()

	output := filepath.Join(t.TempDir(), "asset")
	source, err := DownloadWithFallback(context.Background(), DownloadRequest{
		Output:    output,
		SHA256:    hex.EncodeToString(sum[:]),
		Retries:   0,
		AllowHTTP: true,
	}, []DownloadSource{{URL: failing.URL}, {URL: working.URL}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if source != working.URL {
		t.Fatalf("expected fallback source %s, got %s", working.URL, source)
	}
}
