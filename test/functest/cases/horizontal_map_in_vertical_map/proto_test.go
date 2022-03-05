package horizontalmapinverticalmap

import (
	"testing"

	"github.com/tableauio/tableau"
)

func Test_GenProto(t *testing.T) {
	tableau.GenProto(
		"test",
		"github.com/tableauio/tableau/test/functest/cases/horizontal_map_in_vertical_map/protoconf",
		"./testdata",
		"./proto",
	)
}
