package confgen

import "fmt"

const (
	App     = "confgen"
	Version = "0.4.1"
)

func AppVersion() string {
	return fmt.Sprintf("%s v%s", App, Version)
}
