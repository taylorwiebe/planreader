package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/taylorwiebe/planreader/internal/buildinfo"
)

func TestVersionCommandPrintsBuildIdentity(t *testing.T) {
	oldVersion, oldCommit, oldOrigin := buildinfo.Version, buildinfo.Commit, buildinfo.Origin
	t.Cleanup(func() { buildinfo.Version, buildinfo.Commit, buildinfo.Origin = oldVersion, oldCommit, oldOrigin })
	buildinfo.Version, buildinfo.Commit, buildinfo.Origin = "1.2.3", "abc123", "release"
	var stdout bytes.Buffer
	if err := run([]string{"version"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Planreader 1.2.3", "release", "abc123"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("version output missing %q: %s", want, stdout.String())
		}
	}
}
