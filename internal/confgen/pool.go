package confgen

import (
	"sync"

	"github.com/tableauio/tableau/proto/tableaupb"
)

var fieldOptionsPool *sync.Pool

func init() {
	fieldOptionsPool = &sync.Pool{
		New: func() interface{} {
			return new(tableaupb.FieldOptions)
		},
	}
}
