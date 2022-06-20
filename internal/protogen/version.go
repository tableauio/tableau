package protogen

import "fmt"

const (
	App     = "protogen"
	Version = "0.4.0"
)

func AppVersion() string {
	return fmt.Sprintf("%s v%s", App, Version)
}
