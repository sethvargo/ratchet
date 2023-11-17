package version

import (
	"runtime"
	"runtime/debug"
)

var (
	// Name is the name of the binary.
	name string
	Name = valueOrFallback(name, func() string {
		return "ratchet"
	})

	// Version is the main package version.
	version string
	Version = valueOrFallback(version, func() string {
		if info, ok := debug.ReadBuildInfo(); ok {
			if v := info.Main.Version; v != "" {
				return v // e.g. "v0.0.1-alpha6.0.20230815191505-8628f8201363"
			}
		}

		return "source"
	})

	// Commit is the git sha.
	commit string
	Commit = valueOrFallback(commit, func() string {
		if info, ok := debug.ReadBuildInfo(); ok {
			for _, setting := range info.Settings {
				if setting.Key == "vcs.revision" {
					return setting.Value
				}
			}
		}

		return "HEAD"
	})

	// OSArch returns the denormalized operating system and architecture.
	OSArch = runtime.GOOS + "/" + runtime.GOARCH

	// HumanVersion is the compiled version.
	HumanVersion = Name + " " + Version + " (" + Commit + ", " + OSArch + ")"
)

func valueOrFallback(val string, fn func() string) string {
	if val != "" {
		return val
	}
	return fn()
}
