package release

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCompareVersions(t *testing.T) {
	for _, tc := range []struct {
		current, latest string
		want            int
	}{{"v1.2.0", "v1.3.0", -1}, {"1.2.0", "v1.2.0", 0}, {"v2.0.0", "v1.9.9", 1}} {
		got, err := CompareVersions(tc.current, tc.latest)
		if err != nil || got != tc.want {
			t.Fatalf("CompareVersions(%q,%q)=(%d,%v), want %d", tc.current, tc.latest, got, err, tc.want)
		}
	}
	if _, err := CompareVersions("dev", "v1.0.0"); err == nil {
		t.Fatal("expected malformed version error")
	}
}

func TestClientPinsReleaseAndVerifiesChecksum(t *testing.T) {
	archive := fixtureArchive(t, map[string]string{"planreader": "binary", "lib/libvoice.dylib": "library"}, nil)
	sum := sha256.Sum256(archive)
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var body string
		switch r.URL.Path {
		case "/latest":
			body = `{"tag_name":"v1.2.3","assets":[{"name":"planreader-v1.2.3-darwin-arm64.tar.gz","browser_download_url":"http://fixture/archive"},{"name":"checksums.txt","browser_download_url":"http://fixture/checksums"}]}`
		case "/checksums":
			body = fmt.Sprintf("%x  planreader-v1.2.3-darwin-arm64.tar.gz\n", sum)
		case "/archive":
			body = string(archive)
		default:
			return response(r, 404, ""), nil
		}
		return response(r, 200, body), nil
	})}
	c := Client{HTTP: client, LatestURL: "http://fixture/latest", AllowedHost: "fixture", AllowHTTPForTests: true}
	rel, err := c.Resolve(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rel.Version != "v1.2.3" || !strings.Contains(rel.ArchiveURL, "/archive") {
		t.Fatalf("unexpected release: %#v", rel)
	}
	dir := t.TempDir()
	if err := c.DownloadAndExtract(context.Background(), rel, dir, VerifyOptions{SkipAppleTrustForTests: true}); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "planreader")); string(got) != "binary" {
		t.Fatalf("got %q", got)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func response(r *http.Request, status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Status: fmt.Sprintf("%d", status), Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header), Request: r}
}

func TestArchiveRejectsUnsafeEntries(t *testing.T) {
	for _, entry := range []tar.Header{{Name: "../escape", Mode: 0o644, Typeflag: tar.TypeReg}, {Name: "/escape", Mode: 0o644, Typeflag: tar.TypeReg}, {Name: "planreader", Linkname: "elsewhere", Typeflag: tar.TypeSymlink}} {
		t.Run(entry.Name+string(entry.Typeflag), func(t *testing.T) {
			archive := fixtureArchive(t, nil, &entry)
			if err := Extract(bytes.NewReader(archive), t.TempDir()); err == nil {
				t.Fatal("expected unsafe archive error")
			}
		})
	}
}

func TestCacheExpiresAfterDay(t *testing.T) {
	path := filepath.Join(t.TempDir(), "check.json")
	now := time.Unix(100000, 0)
	if err := WriteCache(path, Check{Latest: "v2.0.0"}, now); err != nil {
		t.Fatal(err)
	}
	if _, ok := ReadCache(path, now.Add(23*time.Hour)); !ok {
		t.Fatal("fresh cache missed")
	}
	if _, ok := ReadCache(path, now.Add(25*time.Hour)); ok {
		t.Fatal("expired cache accepted")
	}
}

func TestReleaseHostAllowlistSupportsGitHubAssetRedirects(t *testing.T) {
	for _, host := range []string{
		"github.com",
		"api.github.com",
		"objects.githubusercontent.com",
		"release-assets.githubusercontent.com",
	} {
		if !allowedReleaseHost(host, "github.com") {
			t.Errorf("host %q was rejected", host)
		}
	}
	for _, host := range []string{"github.example.com", "github.com.evil.test", "raw.githubusercontent.com"} {
		if allowedReleaseHost(host, "github.com") {
			t.Errorf("host %q was accepted", host)
		}
	}
}

func TestUpdateLockIgnoresStaleContents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "update.lock")
	if err := os.WriteFile(path, []byte("99999999\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	lock, err := acquireUpdateLock(path)
	if err != nil {
		t.Fatal(err)
	}
	defer releaseUpdateLock(lock)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != fmt.Sprint(os.Getpid()) {
		t.Fatalf("lock owner = %q", data)
	}
}

func TestUpdateLockRejectsLiveOwner(t *testing.T) {
	path := filepath.Join(t.TempDir(), "update.lock")
	first, err := acquireUpdateLock(path)
	if err != nil {
		t.Fatal(err)
	}
	defer releaseUpdateLock(first)
	if second, err := acquireUpdateLock(path); err == nil {
		releaseUpdateLock(second)
		t.Fatal("expected a live-owner lock error")
	}
}

func fixtureArchive(t *testing.T, files map[string]string, special *tar.Header) []byte {
	t.Helper()
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		h := &tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}
		if strings.HasPrefix(name, "lib/") {
			h.Mode = 0o644
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		_, _ = tw.Write([]byte(body))
	}
	if special != nil {
		_ = tw.WriteHeader(special)
	}
	_ = tw.Close()
	_ = gz.Close()
	return b.Bytes()
}
