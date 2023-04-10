package tableau

import (
	"runtime/debug"

	"github.com/tableauio/tableau/internal/confgen"
	"github.com/tableauio/tableau/internal/protogen"
)

// VersionInfo holds versions of tableau'd main modules and VCS info.
type VersionInfo struct {
	ProtogenVersion string // version of protogen module
	ConfgenVersion  string // version of confgen module
	// VCS info
	Revision     string
	Time         string
	Experimental string
}

const RevisionSize = 7

// GetVersionInfo returns VersionInfo of tableau.
func GetVersionInfo() *VersionInfo {
	info := &VersionInfo{
		ProtogenVersion: protogen.Version,
		ConfgenVersion:  confgen.Version,
	}
	if buildInfo, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range buildInfo.Settings {
			switch setting.Key {
			case "vcs.revision":
				info.Revision = setting.Value
				if len(info.Revision) >= RevisionSize {
					info.Revision = info.Revision[:RevisionSize]
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
