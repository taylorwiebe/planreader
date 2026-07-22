package buildinfo

import "testing"

func TestIdentityDefaultsToSourceBuild(t *testing.T) {
	oldVersion, oldCommit, oldOrigin := Version, Commit, Origin
	t.Cleanup(func() { Version, Commit, Origin = oldVersion, oldCommit, oldOrigin })
	Version, Commit, Origin = "", "", ""
	got := Current()
	if got.Version != "dev" || got.Origin != "source" || got.Commit != "unknown" {
		t.Fatalf("Current() = %#v", got)
	}
}

func TestIdentityUsesInjectedReleaseValues(t *testing.T) {
	oldVersion, oldCommit, oldOrigin := Version, Commit, Origin
	t.Cleanup(func() { Version, Commit, Origin = oldVersion, oldCommit, oldOrigin })
	Version, Commit, Origin = "1.2.3", "abc123", "release"
	got := Current()
	if got.Version != "1.2.3" || got.Origin != "release" || got.Commit != "abc123" {
		t.Fatalf("Current() = %#v", got)
	}
}
