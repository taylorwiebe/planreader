package release

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

const maxReleaseBytes = 256 << 20

type Client struct {
	HTTP              *http.Client
	LatestURL         string
	AllowedHost       string
	AllowHTTPForTests bool
}

type Release struct{ Version, ArchiveURL, ChecksumsURL, ArchiveName string }
type VerifyOptions struct {
	SkipAppleTrustForTests bool
	ExpectedTeamID         string
}

func (c Client) Resolve(ctx context.Context) (Release, error) {
	var result Release
	if c.LatestURL == "" {
		c.LatestURL = "https://api.github.com/repos/taylorwiebe/planreader/releases/latest"
	}
	var payload struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := c.getJSON(ctx, c.LatestURL, &payload); err != nil {
		return result, err
	}
	if _, err := parseVersion(payload.TagName); err != nil {
		return result, fmt.Errorf("invalid release tag: %w", err)
	}
	archiveName := fmt.Sprintf("planreader-%s-darwin-arm64.tar.gz", payload.TagName)
	for _, asset := range payload.Assets {
		switch asset.Name {
		case archiveName:
			result.ArchiveURL, result.ArchiveName = asset.URL, asset.Name
		case "checksums.txt":
			result.ChecksumsURL = asset.URL
		}
	}
	if result.ArchiveURL == "" || result.ChecksumsURL == "" {
		return result, errors.New("release is missing the Apple-silicon archive or checksums")
	}
	result.Version = payload.TagName
	return result, nil
}

func (c Client) DownloadAndExtract(ctx context.Context, release Release, destination string, options VerifyOptions) error {
	manifest, err := c.get(ctx, release.ChecksumsURL, 1<<20)
	if err != nil {
		return err
	}
	expected, err := checksumFor(manifest, release.ArchiveName)
	if err != nil {
		return err
	}
	archive, err := c.get(ctx, release.ArchiveURL, maxReleaseBytes)
	if err != nil {
		return err
	}
	actual := sha256.Sum256(archive)
	if !strings.EqualFold(expected, hex.EncodeToString(actual[:])) {
		return errors.New("release archive checksum mismatch")
	}
	if err := Extract(bytes.NewReader(archive), destination); err != nil {
		return err
	}
	return VerifyAppleTrust(path.Join(destination, "planreader"), options)
}

func (c Client) getJSON(ctx context.Context, endpoint string, target any) error {
	body, err := c.get(ctx, endpoint, 2<<20)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decoding release response: %w", err)
	}
	return nil
}

func (c Client) get(ctx context.Context, endpoint string, limit int64) ([]byte, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "https" && !(c.AllowHTTPForTests && u.Scheme == "http") {
		return nil, errors.New("release downloads require HTTPS")
	}
	allowed := c.AllowedHost
	if allowed == "" {
		allowed = "github.com"
	}
	if !allowedReleaseHost(u.Host, allowed) {
		return nil, fmt.Errorf("release host %q is not allowed", u.Host)
	}
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 15 * time.Second}
	}
	copyClient := *hc
	copyClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return errors.New("too many redirects")
		}
		if req.URL.Scheme != "https" || !allowedReleaseHost(req.URL.Host, allowed) {
			return errors.New("release redirect changed to an unapproved host")
		}
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := copyClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("release server returned %s", resp.Status)
	}
	if resp.ContentLength > limit {
		return nil, errors.New("release response exceeds size limit")
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, errors.New("release response exceeds size limit")
	}
	return body, nil
}

func allowedReleaseHost(host, configured string) bool {
	if configured != "github.com" {
		return host == configured
	}
	switch host {
	case "github.com", "api.github.com", "objects.githubusercontent.com", "release-assets.githubusercontent.com":
		return true
	default:
		return false
	}
}

func checksumFor(body []byte, filename string) (string, error) {
	s := bufio.NewScanner(strings.NewReader(string(body)))
	for s.Scan() {
		fields := strings.Fields(s.Text())
		if len(fields) == 2 && fields[1] == filename {
			if len(fields[0]) != 64 {
				break
			}
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("checksum for %s is missing", filename)
}
