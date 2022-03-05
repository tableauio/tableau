package horizontalmapinverticalmap

import (
	"testing"

	"github.com/tableauio/tableau"
	"github.com/tableauio/tableau/options"
	_ "github.com/tableauio/tableau/test/functest/cases/horizontal_map_in_vertical_map/protoconf"
)

func Test_GenConf(t *testing.T) {
	tableau.GenConf(
		"test",
		"./testdata",
		"./_conf",
		options.LogLevel("debug"),
	)
}
