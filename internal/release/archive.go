package release

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func Extract(source io.Reader, destination string) error {
	gz, err := gzip.NewReader(source)
	if err != nil {
		return fmt.Errorf("opening release archive: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	seen := map[string]bool{}
	files := 0
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		name := filepath.ToSlash(filepath.Clean(h.Name))
		if filepath.IsAbs(h.Name) || name == ".." || strings.HasPrefix(name, "../") || strings.Contains(h.Name, "\\") || seen[name] {
			return fmt.Errorf("unsafe release archive entry %q", h.Name)
		}
		seen[name] = true
		if name == "lib" && h.Typeflag == tar.TypeDir {
			if err := os.MkdirAll(filepath.Join(destination, "lib"), 0o700); err != nil {
				return err
			}
			continue
		}
		allowed := name == "planreader" || (strings.HasPrefix(name, "lib/") && strings.HasSuffix(name, ".dylib") && !strings.Contains(strings.TrimPrefix(name, "lib/"), "/"))
		if !allowed || h.Typeflag != tar.TypeReg || h.Size < 0 || h.Size > maxReleaseBytes || h.Mode&0o022 != 0 {
			return fmt.Errorf("unsupported release archive entry %q", h.Name)
		}
		files++
		if files > 16 {
			return errors.New("release archive has too many files")
		}
		target := filepath.Join(destination, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if name == "planreader" {
			mode = 0o755
		}
		f, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
		if err != nil {
			return err
		}
		_, copyErr := io.CopyN(f, tr, h.Size)
		closeErr := f.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	if !seen["planreader"] {
		return errors.New("release archive is missing planreader")
	}
	return nil
}

func VerifyAppleTrust(binary string, options VerifyOptions) error {
	if options.SkipAppleTrustForTests {
		return nil
	}
	if options.ExpectedTeamID == "" {
		return errors.New("expected Apple Developer Team ID is not configured")
	}
	out, err := exec.Command("codesign", "-dv", "--verbose=4", binary).CombinedOutput()
	if err != nil {
		return fmt.Errorf("verifying Developer ID signature: %w", err)
	}
	if !strings.Contains(string(out), "TeamIdentifier="+options.ExpectedTeamID) {
		return errors.New("release was not signed by the expected Apple Developer Team")
	}
	if out, err = exec.Command("spctl", "--assess", "--type", "execute", "--verbose=2", binary).CombinedOutput(); err != nil {
		return fmt.Errorf("verifying notarization: %s", strings.TrimSpace(string(out)))
	}
	return nil
}
