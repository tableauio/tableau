package load

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/confgen"
	"github.com/tableauio/tableau/internal/importer"
	"github.com/tableauio/tableau/options"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
)

func Load(msg proto.Message, dir string, fmt format.Format) error {
	switch fmt {
	case format.JSON:
		return loadJSON(msg, dir)
	case format.Text:
		return loadText(msg, dir)
	case format.Wire:
		return loadWire(msg, dir)
	case format.Excel, format.CSV, format.XML:
		return loadExcel(msg, dir, fmt)
	default:
		return errors.Errorf("unknown format: %v", fmt)
	}
}

func loadJSON(msg proto.Message, dir string) error {
	msgName := string(msg.ProtoReflect().Descriptor().Name())
	path := filepath.Join(dir, msgName+format.JSONExt)

	if content, err := os.ReadFile(path); err != nil {
		return errors.Wrapf(err, "failed to read file: %v", path)
	} else {
		if err := protojson.Unmarshal(content, msg); err != nil {
			return errors.Wrapf(err, "failed to unmarhsal message: %v", msgName)
		}
	}
	return nil
}

func loadText(msg proto.Message, dir string) error {
	msgName := string(msg.ProtoReflect().Descriptor().Name())
	path := filepath.Join(dir, msgName+format.TextExt)

	if content, err := os.ReadFile(path); err != nil {
		return errors.Wrapf(err, "failed to read file: %v", path)
	} else {
		if err := prototext.Unmarshal(content, msg); err != nil {
			return errors.Wrapf(err, "failed to unmarhsal message: %v", msgName)
		}
	}
	return nil
}

func loadWire(msg proto.Message, dir string) error {
	msgName := string(msg.ProtoReflect().Descriptor().Name())
	path := filepath.Join(dir, msgName+format.WireExt)

	if content, err := os.ReadFile(path); err != nil {
		return errors.Wrapf(err, "failed to read file: %v", path)
	} else {
		if err := proto.Unmarshal(content, msg); err != nil {
			return errors.Wrapf(err, "failed to unmarhsal message: %v", msgName)
		}
	}
	return nil
}

func loadExcel(msg proto.Message, dir string, format format.Format) error {
	md := msg.ProtoReflect().Descriptor()
	protofile, workbook := confgen.ParseFileOptions(md.ParentFile())
	if workbook == nil {
		return errors.Errorf("workbook options not found of protofile: %v", protofile)
	}
	wbPath := filepath.Join(dir, workbook.Name)
	msgName, wsOpts := confgen.ParseMessageOptions(md)
	sheets := []string{wsOpts.Name}

	header := &options.HeaderOption{
		Namerow: wsOpts.Namerow,
		Typerow: wsOpts.Typerow,
		Noterow: wsOpts.Noterow,
		Datarow: wsOpts.Datarow,

		Nameline: wsOpts.Nameline,
		Typeline: wsOpts.Typeline,
	}
	imp := importer.New(
		wbPath,
		importer.Sheets(sheets),
		importer.Format(format),
		importer.Header(header),
	)

	sheet, err := imp.GetSheet(wsOpts.Name)
	if err != nil {
		return errors.WithMessagef(err, "%v|get sheet failed: %s", msgName, wsOpts.Name)
	}
	pkgName := md.ParentFile().Package()
	// TODO: support LocationName setting by using Functional Options
	locationName := ""
	parser := confgen.NewSheetParser(string(pkgName), locationName, wsOpts)
	if err := parser.Parse(msg, sheet); err != nil {
		return errors.WithMessagef(err, "%v|failed to parse sheet: %s", msgName, wsOpts.Name)
	}
	return nil
}
