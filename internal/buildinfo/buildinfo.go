package buildinfo

import "runtime/debug"

var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
	Dirty   = "unknown"
	BuiltBy = "source"
)

type Info struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
	Dirty   string `json:"dirty"`
	BuiltBy string `json:"built_by"`
}

func Current() Info {
	info := Info{
		Version: valueOrDefault(Version, "dev"),
		Commit:  valueOrDefault(Commit, "unknown"),
		Date:    valueOrDefault(Date, "unknown"),
		Dirty:   valueOrDefault(Dirty, "unknown"),
		BuiltBy: valueOrDefault(BuiltBy, "source"),
	}

	if build, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range build.Settings {
			switch setting.Key {
			case "vcs.revision":
				if info.Commit == "unknown" {
					info.Commit = setting.Value
				}
			case "vcs.time":
				if info.Date == "unknown" {
					info.Date = setting.Value
				}
			case "vcs.modified":
				if info.Dirty == "unknown" {
					info.Dirty = setting.Value
				}
			}
		}
	}

	return info
}

func valueOrDefault(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
