package strcase

import (
	"sync"
)

var uppercaseAcronym = sync.Map{}

// ConfigureAcronym allows you to add additional words which will be considered
// as acronyms.
//
// Examples:
//
//	ConfigureAcronym("API", "api")
//	ConfigureAcronym("K8s", "k8s")
//	ConfigureAcronym("3D", "3d")
func ConfigureAcronym(key, val string) {
	uppercaseAcronym.Store(key, val)
}
