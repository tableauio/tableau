package confgen

import "fmt"

const (
	App     = "confgen"
	Version = "0.5.0"
)

func AppVersion() string {
	return fmt.Sprintf("%s v%s", App, Version)
}
