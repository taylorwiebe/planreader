package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const maxModelInstallBytes = 64 << 20

type hfEntry struct {
	Type string `json:"type"`
	Path string `json:"path"`
	Size int64  `json:"size"`
	LFS  *struct {
		OID  string `json:"oid"`
		Size int64  `json:"size"`
	} `json:"lfs"`
}

type installedManifest struct {
	Revision string                   `json:"revision"`
	Files    map[string]fileIntegrity `json:"files"`
}

type fileIntegrity struct {
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type modelInstaller struct {
	client *http.Client
	mu     sync.Mutex
}

func newModelInstaller() *modelInstaller {
	client := &http.Client{Timeout: 15 * time.Minute}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) > 8 || !approvedDownloadHost(req.URL.Hostname()) {
			return errors.New("voice pack download redirected to an unapproved host")
		}
		return nil
	}
	return &modelInstaller{client: client}
}

func approvedDownloadHost(host string) bool {
	return host == "huggingface.co" || host == "cdn-lfs.huggingface.co" || strings.HasSuffix(host, ".xethub.hf.co")
}

func (d *modelInstaller) install(ctx context.Context, store *ModelStore, model VoiceModel) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if store.installed(model.ID) {
		return nil
	}
	if !model.Supported {
		return errors.New("local voice packs currently require a Mac with Apple silicon")
	}
	entries, err := d.entries(ctx, model)
	if err != nil {
		return err
	}
	var total int64
	manifest := installedManifest{Revision: model.Revision, Files: make(map[string]fileIntegrity)}
	for _, entry := range entries {
		if entry.Type == "file" {
			total += entry.Size
		}
	}
	if total <= 0 || total > maxModelInstallBytes {
		return fmt.Errorf("voice pack is an unexpected size (%d bytes)", total)
	}
	modelsDir := filepath.Join(store.root, "models")
	stage, err := os.MkdirTemp(modelsDir, ".install-"+model.ID+"-*")
	if err != nil {
		return fmt.Errorf("preparing voice pack: %w", err)
	}
	defer os.RemoveAll(stage)
	for _, entry := range entries {
		if entry.Type != "file" || entry.Path == ".gitattributes" || entry.Path == "README.md" {
			continue
		}
		if err := safeRelativePath(entry.Path); err != nil {
			return err
		}
		integrity, err := d.downloadFile(ctx, model, entry, stage)
		if err != nil {
			return err
		}
		manifest.Files[entry.Path] = integrity
	}
	for _, required := range requiredModelAssets(model) {
		if _, err := os.Stat(filepath.Join(stage, filepath.FromSlash(required))); err != nil {
			return fmt.Errorf("voice pack is missing %s", required)
		}
	}
	if err := writeJSONAtomic(filepath.Join(stage, ".complete"), manifest); err != nil {
		return err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	final := store.modelDir(model.ID)
	if err := os.RemoveAll(final); err != nil {
		return err
	}
	if err := os.Rename(stage, final); err != nil {
		return fmt.Errorf("activating voice pack: %w", err)
	}
	return nil
}

func safeRelativePath(path string) error {
	clean := filepath.Clean(filepath.FromSlash(path))
	if path == "" || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return errors.New("voice pack contains an unsafe file path")
	}
	return nil
}

func (d *modelInstaller) entries(ctx context.Context, model VoiceModel) ([]hfEntry, error) {
	next := fmt.Sprintf("https://huggingface.co/api/models/%s/tree/%s?recursive=true&expand=true", model.Repository, model.Revision)
	var entries []hfEntry
	for next != "" {
		if err := validateDownloadURL(next); err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, next, nil)
		if err != nil {
			return nil, err
		}
		resp, err := d.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("reading voice pack manifest: %w", err)
		}
		var page []hfEntry
		decodeErr := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&page)
		closeErr := resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("voice pack manifest returned %s", resp.Status)
		}
		if decodeErr != nil {
			return nil, fmt.Errorf("decoding voice pack manifest: %w", decodeErr)
		}
		if closeErr != nil {
			return nil, closeErr
		}
		entries = append(entries, page...)
		next = nextPage(resp.Header.Get("Link"))
		if len(entries) > 1000 {
			return nil, errors.New("voice pack manifest contains too many files")
		}
	}
	return entries, nil
}

func validateDownloadURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.User != nil || !approvedDownloadHost(parsed.Hostname()) {
		return errors.New("voice pack URL is not an approved HTTPS address")
	}
	if port := parsed.Port(); port != "" && port != "443" {
		return errors.New("voice pack URL uses an unapproved port")
	}
	return nil
}

func nextPage(link string) string {
	for _, part := range strings.Split(link, ",") {
		part = strings.TrimSpace(part)
		if !strings.HasSuffix(part, `; rel="next"`) {
			continue
		}
		start, end := strings.IndexByte(part, '<'), strings.IndexByte(part, '>')
		if start == 0 && end > start {
			return part[start+1 : end]
		}
	}
	return ""
}

func (d *modelInstaller) downloadFile(ctx context.Context, model VoiceModel, entry hfEntry, stage string) (fileIntegrity, error) {
	endpoint := fmt.Sprintf("https://huggingface.co/%s/resolve/%s/%s", model.Repository, model.Revision, url.PathEscape(entry.Path))
	endpoint = strings.ReplaceAll(endpoint, "%2F", "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fileIntegrity{}, err
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return fileIntegrity{}, fmt.Errorf("downloading %s: %w", entry.Path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fileIntegrity{}, fmt.Errorf("downloading %s returned %s", entry.Path, resp.Status)
	}
	destination := filepath.Join(stage, filepath.FromSlash(entry.Path))
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return fileIntegrity{}, err
	}
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fileIntegrity{}, err
	}
	hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(file, hash), io.LimitReader(resp.Body, entry.Size+1))
	closeErr := file.Close()
	if copyErr != nil {
		return fileIntegrity{}, fmt.Errorf("saving %s: %w", entry.Path, copyErr)
	}
	if closeErr != nil {
		return fileIntegrity{}, closeErr
	}
	if written != entry.Size {
		return fileIntegrity{}, fmt.Errorf("%s was %d bytes; expected %d", entry.Path, written, entry.Size)
	}
	if entry.LFS != nil {
		got := hex.EncodeToString(hash.Sum(nil))
		want := strings.TrimPrefix(entry.LFS.OID, "sha256:")
		if got != want {
			return fileIntegrity{}, fmt.Errorf("%s failed its integrity check", entry.Path)
		}
	}
	return fileIntegrity{Size: written, SHA256: hex.EncodeToString(hash.Sum(nil))}, nil
}
