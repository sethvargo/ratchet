package version

var (
	// Name is the name of the binary.
	Name = "ratchet"

	// Version is the main package version.
	Version = "unknown"

	// Commit is the git sha.
	Commit = "unknown"

	// HumanVersion is the compiled version.
	HumanVersion = Name + " v" + Version + " (" + Commit + ")"
)
