package launchpad

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type DownloadRequest struct {
	URL       string
	Output    string
	SHA256    string
	ProxyURL  string
	Retries   int
	AllowHTTP bool
}

type DownloadSource struct {
	URL      string
	ProxyURL string
}

func DownloadWithFallback(ctx context.Context, req DownloadRequest, sources []DownloadSource, progress func(received, total int64)) (string, error) {
	if len(sources) == 0 {
		return "", errors.New("at least one download source is required")
	}
	var failures []string
	for index, source := range sources {
		if index > 0 {
			_ = os.Remove(req.Output + ".part")
		}
		attempt := req
		attempt.URL = source.URL
		attempt.ProxyURL = source.ProxyURL
		if err := DownloadVerified(ctx, attempt, progress); err == nil {
			return source.URL, nil
		} else {
			failures = append(failures, fmt.Sprintf("%s: %v", source.URL, err))
		}
	}
	return "", fmt.Errorf("all download sources failed: %s", strings.Join(failures, "; "))
}

func DownloadVerified(ctx context.Context, req DownloadRequest, progress func(received, total int64)) error {
	u, err := url.Parse(req.URL)
	if err != nil {
		return err
	}
	if u.Scheme != "https" && !req.AllowHTTP {
		return errors.New("downloads require HTTPS")
	}
	if len(req.SHA256) != 64 {
		return errors.New("an expected SHA-256 is required")
	}
	if err := os.MkdirAll(filepath.Dir(req.Output), 0o755); err != nil {
		return err
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if req.ProxyURL != "" {
		proxy, err := url.Parse(req.ProxyURL)
		if err != nil {
			return err
		}
		transport.Proxy = http.ProxyURL(proxy)
	}
	client := &http.Client{Transport: transport, Timeout: 0}
	partial := req.Output + ".part"
	var lastErr error
	for attempt := 0; attempt <= req.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(1<<min(attempt, 5)) * time.Second):
			}
		}
		if err := downloadAttempt(ctx, client, req.URL, partial, progress); err != nil {
			lastErr = err
			continue
		}
		actual, err := FileSHA256(partial)
		if err != nil {
			lastErr = err
			continue
		}
		if !strings.EqualFold(actual, req.SHA256) {
			_ = os.Remove(partial)
			return fmt.Errorf("SHA-256 mismatch: expected %s, got %s", req.SHA256, actual)
		}
		return os.Rename(partial, req.Output)
	}
	return fmt.Errorf("download failed after %d attempt(s): %w", req.Retries+1, lastErr)
}

func downloadAttempt(ctx context.Context, client *http.Client, rawURL, output string, progress func(received, total int64)) error {
	var offset int64
	if info, err := os.Stat(output); err == nil {
		offset = info.Size()
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	if offset > 0 {
		request.Header.Set("Range", "bytes="+strconv.FormatInt(offset, 10)+"-")
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("unexpected HTTP status %s", response.Status)
	}
	flags := os.O_CREATE | os.O_WRONLY
	if response.StatusCode == http.StatusPartialContent {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
		offset = 0
	}
	file, err := os.OpenFile(output, flags, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	total := response.ContentLength
	if total >= 0 {
		total += offset
	}
	reader := &progressReader{Reader: response.Body, received: offset, total: total, callback: progress}
	_, err = io.Copy(file, reader)
	return err
}

func FileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

type progressReader struct {
	io.Reader
	received int64
	total    int64
	callback func(int64, int64)
}

func (r *progressReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	r.received += int64(n)
	if r.callback != nil {
		r.callback(r.received, r.total)
	}
	return n, err
}
