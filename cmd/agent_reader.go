package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const agentReaderFile = "agent-reader-url"

func claimAgentReader(readerURL string) (func(), error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return nil, fmt.Errorf("finding cache directory: %w", err)
	}
	return claimAgentReaderAt(filepath.Join(cacheDir, "planreader", agentReaderFile), readerURL, shutdownAgentReader)
}

func claimAgentReaderAt(registryPath, readerURL string, stopPrevious func(string)) (func(), error) {
	readerURL = strings.TrimSpace(readerURL)
	if readerURL == "" {
		return nil, errors.New("reader URL is empty")
	}
	if previous, err := os.ReadFile(registryPath); err == nil {
		stopPrevious(strings.TrimSpace(string(previous)))
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("reading agent reader registry: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(registryPath), 0o700); err != nil {
		return nil, fmt.Errorf("creating agent reader registry: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(registryPath), ".agent-reader-*")
	if err != nil {
		return nil, fmt.Errorf("creating agent reader registry: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return nil, fmt.Errorf("securing agent reader registry: %w", err)
	}
	if _, err := io.Copy(temporary, bytes.NewBufferString(readerURL)); err != nil {
		temporary.Close()
		return nil, fmt.Errorf("writing agent reader registry: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return nil, fmt.Errorf("closing agent reader registry: %w", err)
	}
	if err := os.Rename(temporaryPath, registryPath); err != nil {
		return nil, fmt.Errorf("publishing agent reader registry: %w", err)
	}

	return func() {
		current, err := os.ReadFile(registryPath)
		if err == nil && strings.TrimSpace(string(current)) == readerURL {
			_ = os.Remove(registryPath)
		}
	}, nil
}

func shutdownAgentReader(readerURL string) {
	if readerURL == "" {
		return
	}
	parsed, err := url.Parse(readerURL)
	if err != nil || parsed.Scheme != "http" || parsed.Hostname() != "127.0.0.1" || !strings.HasPrefix(parsed.Path, "/reader/") {
		return
	}
	client := &http.Client{Timeout: time.Second}
	request, err := http.NewRequest(http.MethodPost, strings.TrimRight(parsed.String(), "/")+"/api/agent/shutdown", nil)
	if err != nil {
		return
	}
	response, err := client.Do(request)
	if err == nil {
		_ = response.Body.Close()
	}
}
