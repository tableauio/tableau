package bench

import (
	"os"
	"strconv"
	"testing"

	"github.com/tableauio/tableau"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/x/xfs"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/xerrors"
)

// cmd: go test -run ^Test_genConf$ -cpuprofile=cpu.prof
// cmd: go test -run ^Test_genConf$ -memprofile=mem.prof
// cmd: go tool pprof -http :8888 cpu.prof
// cmd: go tool pprof -http :9999 mem.prof
func Test_genConf(t *testing.T) {
	// NOTE: generate testdata ahead
	genTestdata(10)

	if err := genConf("INFO"); err != nil {
		t.Errorf("%+v", err)
		t.Fatalf("%s", xerrors.NewDesc(err))
	}
}

func genConf(logLevel string) error {
	return tableau.GenConf(
		"protoconf",
		"./testdata",
		"./_conf",
		options.LocationName("Asia/Shanghai"),
		options.Lang("zh"),
		options.Conf(
			&options.ConfOption{
				Input: &options.ConfInputOption{
					ProtoPaths: []string{"./proto", "."},
					ProtoFiles: []string{"./proto/*.proto"},
				},
				Output: &options.ConfOutputOption{
					Pretty:          true,
					Formats:         []format.Format{format.JSON},
					EmitUnpopulated: true,
				},
			},
		),
		options.Log(
			&log.Options{
				Level: logLevel,
				Mode:  "FULL",
			},
		),
	)
}

func genTestdata(maxID int) {
	if ok, err := xfs.Exists("testdata"); err != nil {
		panic(err)
	} else if !ok {
		// create output dir
		err = os.MkdirAll("testdata", xfs.DefaultDirPerm)
		if err != nil {
			panic(err)
		}
	}
	b := book.NewBook("Test", "testdata/Test.xlsx", nil)
	// add metasheet
	metasheet := book.NewTableSheet(book.MetasheetName, nil)
	b.AddSheet(metasheet)
	// add Item sheet
	rows := [][]string{
		// header
		{"ID", "Name", "PropID", "PropValue", "Cost1ID", "Cost1Value", "Cost2ID", "Cost2Value"},
		{"map<uint32, Item>", "string", "[Prop]int32", "int64", "[Prop]int32", "int64", "int32", "int64"},
		{"Item's ID", "Item's name", "Prop's ID", "Prop's value", "Cost1's ID", "Cost1's value", "Cost2's ID", "Cost2's value"},
		// data
		{"1", "Apple", "1", "10", "1001", "10000", "1002", "20000"},
		{"2", "Orange", "1", "20", "1001", "10000", "1002", "20000"},
		{"2", "Banana", "2", "30", "1001", "10000", "1002", "20000"},
	}

	// add big data
	for i := 3; i <= maxID; i++ {
		id := strconv.FormatInt(int64(i), 10)
		newRows := [][]string{
			{id, "Orange", "1", "20", "1001", "10000", "1002", "20000"},
			{id, "Banana", "2", "30", "1001", "10000", "1002", "20000"},
		}
		rows = append(rows, newRows...)
	}

	itemSheet := book.NewTableSheet("Item", rows)
	b.AddSheet(itemSheet)

	err := b.ExportExcel()
	if err != nil {
		panic(err)
	}
}

func Test_genProto(t *testing.T) {
	err := genProto("DEBUG")
	if err != nil {
		t.Errorf("%+v", err)
		t.Fatalf("%s", xerrors.NewDesc(err))
	}
}

func genProto(logLevel string) error {
	// prepare output common dir
	return tableau.GenProto(
		"protoconf",
		"./testdata",
		"./_proto",
		options.Proto(
			&options.ProtoOption{
				Input: &options.ProtoInputOption{
					ProtoPaths: []string{"./_proto"},
					Formats: []format.Format{
						format.Excel,
					},
					Header: &options.HeaderOption{
						NameRow: 1,
						TypeRow: 2,
						NoteRow: 3,
						DataRow: 4,
					},
				},
				Output: &options.ProtoOutputOption{
					FileOptions: map[string]string{
						"go_package": "github.com/tableauio/tableau/test/bench/protoconf",
					},
				},
			},
		),
		options.Log(
			&log.Options{
				Level: logLevel,
				Mode:  "FULL",
			},
		),
		// options.Lang("zh"),
	)
}
