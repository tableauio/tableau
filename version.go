package tableau

import (
	"runtime/debug"

	"github.com/tableauio/tableau/internal/confgen"
	"github.com/tableauio/tableau/internal/protogen"
)

const version = "0.15.0" // version of tableau
const revisionSize = 7

// VersionInfo holds versions of tableau's main modules and VCS info.
type VersionInfo struct {
	Version         string // version of tableau
	ProtogenVersion string // version of protogen module
	ConfgenVersion  string // version of confgen module
	// VCS info
	Revision     string
	Time         string
	Experimental string
}

// GetVersionInfo returns VersionInfo of tableau.
func GetVersionInfo() *VersionInfo {
	info := &VersionInfo{
		Version:         version,
		ProtogenVersion: protogen.Version,
		ConfgenVersion:  confgen.Version,
	}
	if buildInfo, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range buildInfo.Settings {
			switch setting.Key {
			case "vcs.revision":
				info.Revision = setting.Value
				if len(info.Revision) >= revisionSize {
					info.Revision = info.Revision[:revisionSize]
				}
			case "vcs.time":
				info.Time = setting.Value
			case "vcs.modified":
				info.Experimental = setting.Value
			}
		}
	}
	return info
}
