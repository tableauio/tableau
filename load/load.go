// Package load provides functions to load a protobuf message from
// different formats:
//   - generated fomats: json, bin, txt
//   - origin formats: xlsx, csv, xml, yaml.
package load

import (
	"os"
	"path/filepath"

	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/confgen"
	"github.com/tableauio/tableau/internal/importer"
	"github.com/tableauio/tableau/internal/xfs"
	"github.com/tableauio/tableau/internal/xproto"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/proto/tableaupb"
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
	md := msg.ProtoReflect().Descriptor()
	name := string(md.Name())
	var path string
	opts := ParseOptions(options...)
	if p, ok := opts.Paths[name]; ok {
		// path specified in Paths, then use it instead of dir.
		path = p
		fmt = format.GetFormat(p)
	} else {
		// path in dir
		path = filepath.Join(dir, name+format.Format2Ext(fmt))
	}
	_, sheetOpts := confgen.ParseMessageOptions(md)
	if sheetOpts.Patch != tableaupb.Patch_PATCH_NONE {
		return loadWithPatch(msg, path, fmt, sheetOpts.Patch, opts)
	}
	return load(msg, path, fmt, opts)
}

func loadWithPatch(msg proto.Message, path string, fmt format.Format, patch tableaupb.Patch, opts *Options) error {
	if opts.Mode == ModeOnlyMain {
		// ignore patch files when ModeOnlyMain assigned
		return load(msg, path, fmt, opts)
	}
	name := string(msg.ProtoReflect().Descriptor().Name())
	var patchPaths []string
	if p, ok := opts.PatchPaths[name]; ok {
		// patch path specified in PatchPaths, then use it instead of PatchDirs.
		patchPaths = p
	} else {
		// patch path in PatchDirs
		for _, patchDir := range opts.PatchDirs {
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
		if opts.Mode == ModeOnlyPatch {
			// just returns empty message when ModeOnlyPatch assigned but no valid patch file provided.
			return nil
		}
		// no valid patch path provided, then just load from the "main" file.
		return load(msg, path, fmt, opts)
	}

	switch patch {
	case tableaupb.Patch_PATCH_REPLACE:
		// just use the last "patch" file
		patchPath := existedPatchPaths[len(existedPatchPaths)-1]
		if err := load(msg, patchPath, format.GetFormat(patchPath), opts); err != nil {
			return err
		}
	case tableaupb.Patch_PATCH_MERGE:
		if opts.Mode != ModeOnlyPatch {
			// load msg from the "main" file
			if err := load(msg, path, fmt, opts); err != nil {
				return err
			}
		}
		patchMsg := msg.ProtoReflect().New().Interface()
		// load patchMsg from each "patch" file
		for _, patchPath := range existedPatchPaths {
			if err := load(patchMsg, patchPath, format.GetFormat(patchPath), opts); err != nil {
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

// load loads the generated config file (json/text/bin) from the given
// directory.
func load(msg proto.Message, path string, fmt format.Format, opts *Options) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return xerrors.Wrapf(err, "failed to read file: %v", path)
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
func loadOrigin(msg proto.Message, dir string, options ...Option) error {
	opts := ParseOptions(options...)

	md := msg.ProtoReflect().Descriptor()
	protofile, workbook := confgen.ParseFileOptions(md.ParentFile())
	if workbook == nil {
		return xerrors.Errorf("workbook options not found of protofile: %v", protofile)
	}
	// rewrite subdir
	rewrittenWorkbookName := xfs.RewriteSubdir(workbook.Name, opts.SubdirRewrites)
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
		return xerrors.Wrapf(err, "failed to import workbook: %v", wbPath)
	}

	// get merger importer infos
	impInfos, err := importer.GetMergerImporters(dir, workbook.Name, wsOpts.Name, wsOpts.Merger, opts.SubdirRewrites)
	if err != nil {
		return xerrors.Wrapf(err, "failed to get merger importer infos for %s", wbPath)
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
