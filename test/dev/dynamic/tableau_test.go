package dynamic

import (
	"testing"

	"github.com/tableauio/tableau"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/options"
)

func Test_GenJSON(t *testing.T) {
	tableau.GenConf(
		"protoconf",
		"../testdata",
		"../_conf",
		options.ImportPaths("../proto"),
		options.ProtoFiles("../proto/*.proto"),
		options.LogLevel("debug"),
		options.OutputFormats(format.JSON),
	)
}
