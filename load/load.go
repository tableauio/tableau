// Package load provides functions to load a protobuf message from
// different formats:
//   - generated fomats: json, bin, txt
//   - origin formats: xlsx, csv, xml, yaml.
package load

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/confgen"
	"github.com/tableauio/tableau/internal/fs"
	"github.com/tableauio/tableau/internal/importer"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/xerrors"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// Load fills message from file in the specified directory and format.
func Load(msg proto.Message, dir string, fmt format.Format, options ...Option) error {
	if format.IsInputFormat(fmt) {
		return loadOrigin(msg, dir, options...)
	}
	name := string(msg.ProtoReflect().Descriptor().Name())
	path := filepath.Join(dir, name+format.Format2Ext(fmt))
	opts := ParseOptions(options...)
	if opts.Paths != nil {
		if p, ok := opts.Paths[name]; ok {
			// path specified explicitly, then use it directly
			path = p
			fmt = format.GetFormat(path)
		}
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return errors.Wrapf(err, "failed to read file: %v", path)
	}

	var unmarshalErr error
	switch fmt {
	case format.JSON:
		unmarshalOpts := protojson.UnmarshalOptions{
			DiscardUnknown: opts.IgnoreUnknownFields,
		}
		unmarshalErr = unmarshalOpts.Unmarshal(content, msg)
	case format.Text:
		unmarshalErr = prototext.Unmarshal(content, msg)
	case format.Bin:
		unmarshalErr = proto.Unmarshal(content, msg)
	default:
		return errors.Errorf("unknown format: %v", fmt)
	}
	if unmarshalErr != nil {
		lines := extractLinesOnUnmarshalError(unmarshalErr, fmt, content)
		return xerrors.E0002(path, name, unmarshalErr.Error(), lines)
	}
	return nil
}

// loadOrigin loads the origin file (excel/csv/xml/yaml) from the given
// directory.
func loadOrigin(msg proto.Message, dir string, options ...Option) error {
	opts := ParseOptions(options...)

	md := msg.ProtoReflect().Descriptor()
	protofile, workbook := confgen.ParseFileOptions(md.ParentFile())
	if workbook == nil {
		return errors.Errorf("workbook options not found of protofile: %v", protofile)
	}
	// rewrite subdir
	rewrittenWorkbookName := fs.RewriteSubdir(workbook.Name, opts.SubdirRewrites)
	wbPath := filepath.Join(dir, rewrittenWorkbookName)
	log.Debugf("load origin file: %v", wbPath)
	// get sheet name
	_, wsOpts := confgen.ParseMessageOptions(md)
	sheets := []string{wsOpts.Name}

	self, err := importer.New(
		wbPath,
		importer.Sheets(sheets),
	)
	if err != nil {
		return errors.WithMessagef(err, "failed to import workbook: %v", wbPath)
	}

	// get merger importer infos
	impInfos, err := importer.GetMergerImporters(dir, workbook.Name, wsOpts.Name, wsOpts.Merger, opts.SubdirRewrites)
	if err != nil {
		return errors.WithMessagef(err, "failed to get merger importer infos for %s", wbPath)
	}
	// append self
	impInfos = append(impInfos, importer.ImporterInfo{Importer: self})

	sheetInfo := &confgen.SheetInfo{
		ProtoPackage:    string(md.ParentFile().Package()),
		LocationName:    opts.LocationName,
		PrimaryBookName: workbook.Name,
		MD:              md,
		Opts:            wsOpts,
		ExtInfo: &confgen.SheetParserExtInfo{
			InputDir:       dir,
			SubdirRewrites: opts.SubdirRewrites,
			PRFiles:        protoregistry.GlobalFiles,
			BookFormat:     self.Format(),
		},
	}
	protomsg, err := confgen.ParseMessage(sheetInfo, impInfos...)
	if err != nil {
		return err
	}
	// NOTE: deep copy
	proto.Merge(msg, protomsg)
	return nil
}
