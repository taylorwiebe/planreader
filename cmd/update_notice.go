package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/taylorwiebe/planreader/internal/buildinfo"
	planrelease "github.com/taylorwiebe/planreader/internal/release"
)

func startUpdateNotice(output io.Writer) {
	id := buildinfo.Current()
	if id.Origin != "release" {
		return
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return
	}
	cachePath := filepath.Join(configDir, "Planreader", "update-check.json")
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		latest, available := availableUpdate(ctx, id.Version, cachePath, planrelease.Client{}, time.Now())
		if available {
			fmt.Fprintf(output, "Planreader %s is available. Run planreader update when you are ready.\n", latest)
		}
	}()
}

func availableUpdate(ctx context.Context, current, cachePath string, client planrelease.Client, now time.Time) (string, bool) {
	check, fresh := planrelease.ReadCache(cachePath, now)
	if !fresh {
		resolved, err := client.Resolve(ctx)
		if err != nil {
			return "", false
		}
		check = planrelease.Check{Latest: resolved.Version}
		if planrelease.WriteCache(cachePath, check, now) != nil {
			return "", false
		}
	}
	comparison, err := planrelease.CompareVersions(current, check.Latest)
	return check.Latest, err == nil && comparison < 0
}
