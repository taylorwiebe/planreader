package cmd

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	planrelease "github.com/taylorwiebe/planreader/internal/release"
)

func TestAvailableUpdateUsesFreshCache(t *testing.T) {
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	cache := filepath.Join(t.TempDir(), "update.json")
	if err := planrelease.WriteCache(cache, planrelease.Check{Latest: "v1.2.0"}, now); err != nil {
		t.Fatal(err)
	}
	latest, available := availableUpdate(context.Background(), "v1.1.0", cache, planrelease.Client{}, now.Add(time.Hour))
	if latest != "v1.2.0" || !available {
		t.Fatalf("availableUpdate() = %q, %v", latest, available)
	}
}

func TestAvailableUpdateIgnoresCurrentVersion(t *testing.T) {
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	cache := filepath.Join(t.TempDir(), "update.json")
	if err := planrelease.WriteCache(cache, planrelease.Check{Latest: "v1.2.0"}, now); err != nil {
		t.Fatal(err)
	}
	_, available := availableUpdate(context.Background(), "v1.2.0", cache, planrelease.Client{}, now)
	if available {
		t.Fatal("current version was reported as an update")
	}
}
