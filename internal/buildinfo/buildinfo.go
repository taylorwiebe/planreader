package buildinfo

var Version = "dev"
var Commit = "unknown"
var Origin = "source"
var TeamID = ""

type Identity struct{ Version, Commit, Origin string }

func Current() Identity {
	id := Identity{Version, Commit, Origin}
	if id.Version == "" {
		id.Version = "dev"
	}
	if id.Commit == "" {
		id.Commit = "unknown"
	}
	if id.Origin == "" {
		id.Origin = "source"
	}
	return id
}
