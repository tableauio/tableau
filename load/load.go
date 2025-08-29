// Package load provides functions to load a protobuf message from
// different formats:
//   - output formats: JSON, Bin, Text
//   - input formats: Excel, CSV, XML, YAML
package load

import (
	"context"
	"path/filepath"

	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/confgen"
	"github.com/tableauio/tableau/internal/importer"
	"github.com/tableauio/tableau/internal/x/xfs"
	"github.com/tableauio/tableau/internal/x/xproto"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/xerrors"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// LoadMessagerInDir loads message's content in the given dir, based on format and messager options.
func LoadMessagerInDir(msg proto.Message, dir string, fmt format.Format, opts *MessagerOptions) error {
	if format.IsInputFormat(fmt) {
		return loadOrigin(msg, dir, opts)
	}
	md := msg.ProtoReflect().Descriptor()
	name := string(md.Name())
	var path string
	if opts.GetPath() != "" {
		// path specified directly, then use it instead of dir.
		path = opts.GetPath()
		fmt = format.GetFormat(path)
	} else {
		// path in dir
		path = filepath.Join(dir, name+format.Format2Ext(fmt))
	}
	_, sheetOpts := confgen.ParseMessageOptions(md)
	if sheetOpts.GetPatch() != tableaupb.Patch_PATCH_NONE {
		return loadMessagerWithPatch(msg, path, fmt, sheetOpts.GetPatch(), opts)
	}
	return opts.GetLoadFunc()(msg, path, fmt, opts)
}

// LoadMessager is the default [LoadFunc] which loads the message's content
// based on the given path, format, and options.
//
// NOTE: only output formats (JSON, Bin, Text) are supported.
func LoadMessager(msg proto.Message, path string, fmt format.Format, opts *MessagerOptions) error {
	content, err := opts.GetReadFunc()(path)
	if err != nil {
		return xerrors.Wrapf(err, "failed to read file: %v", path)
	}
	return Unmarshal(content, msg, path, fmt, opts)
}

func loadMessagerWithPatch(msg proto.Message, path string, fmt format.Format, patch tableaupb.Patch, opts *MessagerOptions) error {
	name := string(msg.ProtoReflect().Descriptor().Name())
	mode := opts.GetMode()
	loadFunc := opts.GetLoadFunc()
	if mode == ModeOnlyMain {
		// ignore patch files when ModeOnlyMain specified
		return loadFunc(msg, path, fmt, opts)
	}
	var patchPaths []string
	if opts.GetPatchPaths() != nil {
		// patch path specified in PatchPaths, then use it instead of PatchDirs.
		patchPaths = opts.GetPatchPaths()
	} else {
		// patch path in PatchDirs
		for _, patchDir := range opts.GetPatchDirs() {
			patchPaths = append(patchPaths, filepath.Join(patchDir, name+format.Format2Ext(fmt)))
		}
	}

	// check existence of each patch path
	var existedPatchPaths []string
	for _, patchPath := range patchPaths {
		existed, err := xfs.Exists(patchPath)
		if err != nil {
			return xerrors.Wrapf(err, "failed to check file existence: %s", patchPath)
		}
		if existed {
			existedPatchPaths = append(existedPatchPaths, patchPath)
		}
	}
	if len(existedPatchPaths) == 0 {
		if mode == ModeOnlyPatch {
			// just returns empty message when ModeOnlyPatch specified but no valid patch file provided.
			return nil
		}
		// no valid patch path provided, then just load from the "main" file.
		return loadFunc(msg, path, fmt, opts)
	}

	switch patch {
	case tableaupb.Patch_PATCH_REPLACE:
		// just use the last "patch" file
		patchPath := existedPatchPaths[len(existedPatchPaths)-1]
		if err := loadFunc(msg, patchPath, format.GetFormat(patchPath), opts); err != nil {
			return err
		}
	case tableaupb.Patch_PATCH_MERGE:
		if mode != ModeOnlyPatch {
			// load msg from the "main" file
			if err := loadFunc(msg, path, fmt, opts); err != nil {
				return err
			}
		}
		patchMsg := msg.ProtoReflect().New().Interface()
		// load patchMsg from each "patch" file
		for _, patchPath := range existedPatchPaths {
			if err := loadFunc(patchMsg, patchPath, format.GetFormat(patchPath), opts); err != nil {
				return err
			}
			if err := xproto.PatchMessage(msg, patchMsg); err != nil {
				return err
			}
		}
	default:
		return xerrors.Errorf("unknown patch type: %v", patch)
	}
	log.Debugf("patched(%s) %s by %v: %s", patch, name, existedPatchPaths, msg)
	return nil
}

// Unmarshal unmarshals the message based on the given content, format, and options.
//
// NOTE: only output formats (JSON, Bin, Text) are supported.
func Unmarshal(content []byte, msg proto.Message, path string, fmt format.Format, opts *MessagerOptions) error {
	var unmarshalErr error
	switch fmt {
	case format.JSON:
		unmarshalOpts := protojson.UnmarshalOptions{
			DiscardUnknown: opts.GetIgnoreUnknownFields(),
		}
		unmarshalErr = unmarshalOpts.Unmarshal(content, msg)
	case format.Text:
		unmarshalErr = prototext.Unmarshal(content, msg)
	case format.Bin:
		unmarshalErr = proto.Unmarshal(content, msg)
	default:
		return xerrors.Errorf("unknown format: %v", fmt)
	}
	if unmarshalErr != nil {
		lines := extractLinesOnUnmarshalError(unmarshalErr, fmt, content)
		fullName := msg.ProtoReflect().Descriptor().FullName()
		return xerrors.E0002(path, string(fullName), unmarshalErr.Error(), lines)
	}
	return nil
}

// loadOrigin loads the origin file (excel/csv/xml/yaml) from the given
// directory.
func loadOrigin(msg proto.Message, dir string, opts *MessagerOptions) error {
	md := msg.ProtoReflect().Descriptor()
	protofile, bookOpts := confgen.ParseFileOptions(md.ParentFile())
	if bookOpts == nil {
		return xerrors.Errorf("workbook options not found of protofile: %v", protofile)
	}
	// rewrite subdir
	bookName := bookOpts.GetName()
	subdirRewrites := opts.GetSubdirRewrites()
	rewrittenWorkbookName := xfs.RewriteSubdir(bookName, subdirRewrites)
	wbPath := filepath.Join(dir, rewrittenWorkbookName)
	log.Debugf("load origin file: %v", wbPath)
	// get sheet name
	_, sheetOpts := confgen.ParseMessageOptions(md)
	sheetName := sheetOpts.GetName()
	sheets := []string{sheetName}

	self, err := importer.New(
		context.Background(),
		wbPath,
		importer.Sheets(sheets),
	)
	if err != nil {
		return xerrors.Wrapf(err, "failed to import workbook: %v", wbPath)
	}

	// get merger importer infos
	impInfos, err := importer.GetMergerImporters(context.Background(), dir, bookName, sheetName, sheetOpts.GetMerger(), subdirRewrites)
	if err != nil {
		return xerrors.Wrapf(err, "failed to get merger importer infos for %s", wbPath)
	}
	// append self
	impInfos = append(impInfos, importer.ImporterInfo{Importer: self})

	sheetInfo := &confgen.SheetInfo{
		ProtoPackage:    string(md.ParentFile().Package()),
		LocationName:    opts.GetLocationName(),
		PrimaryBookName: bookName,
		MD:              md,
		BookOpts:        bookOpts,
		SheetOpts:       sheetOpts,
		ExtInfo: &confgen.SheetParserExtInfo{
			InputDir:       dir,
			SubdirRewrites: subdirRewrites,
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
