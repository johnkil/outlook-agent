package buildinfo

import (
	"runtime/debug"
	"testing"
)

func TestCurrentUsesEmbeddedModuleVersionWhenLDFlagsLeaveDev(t *testing.T) {
	oldVersion := Version
	oldReadBuildInfo := readBuildInfo
	t.Cleanup(func() {
		Version = oldVersion
		readBuildInfo = oldReadBuildInfo
	})

	Version = "dev"
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{
			Main: debug.Module{
				Version: "v9.8.7",
			},
		}, true
	}

	info := Current()

	if info.Version != "v9.8.7" {
		t.Fatalf("expected embedded module version, got %q", info.Version)
	}
}
