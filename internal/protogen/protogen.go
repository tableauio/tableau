package protogen

import (
	iofs "io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/confgen"
	"github.com/tableauio/tableau/internal/excel"
	"github.com/tableauio/tableau/internal/fs"
	"github.com/tableauio/tableau/internal/importer"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/protogen/parseroptions"
	"github.com/tableauio/tableau/internal/types"
	"github.com/tableauio/tableau/internal/xproto"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/xerrors"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/reflect/protoregistry"
)

const (
	TableauProtoPackage = "tableau"
)

type Generator struct {
	ProtoPackage string // protobuf package name.
	InputDir     string // input dir of workbooks.
	OutputDir    string // output dir of generated protoconf files.

	LocationName string // TZ location name.
	InputOpt     *options.ProtoInputOption
	OutputOpt    *options.ProtoOutputOption

	// internal
	protofiles *protoregistry.Files // all parsed imported proto file descriptors.
	typeInfos  *xproto.TypeInfos    // predefined type infos

	cacheMu           sync.RWMutex                 // guard fields below
	cachedImporters   map[string]importer.Importer // absolute file path -> importer
	cachedBookParsers map[string]*bookParser       // absolute file path -> bookParser
}

func NewGenerator(protoPackage, indir, outdir string, setters ...options.Option) *Generator {
	opts := options.ParseOptions(setters...)
	return NewGeneratorWithOptions(protoPackage, indir, outdir, opts)
}

func NewGeneratorWithOptions(protoPackage, indir, outdir string, opts *options.Options) *Generator {
	g := &Generator{
		ProtoPackage: protoPackage,
		InputDir:     indir,
		OutputDir:    outdir,
		LocationName: opts.LocationName,
		InputOpt:     opts.Proto.Input,
		OutputOpt:    opts.Proto.Output,

		cachedImporters:   make(map[string]importer.Importer),
		cachedBookParsers: make(map[string]*bookParser),
	}

	if opts.Proto.Input.MetasheetName != "" {
		book.SetMetasheetName(opts.Proto.Input.MetasheetName)
	}

	// parse custom imported proto files
	protofiles, err := xproto.ParseProtos(
		g.InputOpt.ProtoPaths,
		g.InputOpt.ProtoFiles...)
	if err != nil {
		log.Panic(err)
	}
	g.protofiles = protofiles
	g.typeInfos = xproto.GetAllTypeInfo(protofiles, g.ProtoPackage)

	return g
}

func (gen *Generator) Generate(relWorkbookPaths ...string) error {
	if len(relWorkbookPaths) == 0 {
		return gen.GenAll()
	}
	return gen.GenWorkbook(relWorkbookPaths...)
}

func (gen *Generator) GenAll() error {
	outputProtoDir := filepath.Join(gen.OutputDir, gen.OutputOpt.Subdir)
	if err := prepareOutdir(outputProtoDir, gen.InputOpt.ProtoFiles, true); err != nil {
		return err
	}

	// first pass
	if len(gen.InputOpt.Subdirs) != 0 {
		for _, subdir := range gen.InputOpt.Subdirs {
			dir := filepath.Join(gen.InputDir, subdir)
			if err := gen.generate(dir); err != nil {
				return err
			}
		}
	} else {
		if err := gen.generate(gen.InputDir); err != nil {
			return err
		}
	}
	// second pass
	return gen.processSecondPass()
}

func (gen *Generator) GenWorkbook(relWorkbookPaths ...string) error {
	outputProtoDir := filepath.Join(gen.OutputDir, gen.OutputOpt.Subdir)
	if err := prepareOutdir(outputProtoDir, gen.InputOpt.ProtoFiles, false); err != nil {
		return err
	}

	// first pass
	var eg1 errgroup.Group
	for _, relWorkbookPath := range relWorkbookPaths {
		absPath := filepath.Join(gen.InputDir, relWorkbookPath)
		eg1.Go(func() error {
			return gen.convertWithErrorModule(filepath.Dir(absPath), filepath.Base(absPath), false, firstPass)
		})
	}
	if err := eg1.Wait(); err != nil {
		return err
	}

	// second pass
	return gen.processSecondPass()
}

func (gen *Generator) processSecondPass() error {
	// second pass
	gen.cacheMu.RLock()
	absPaths := []string{}
	for absPath, _ := range gen.cachedImporters {
		absPaths = append(absPaths, absPath)
	}
	gen.cacheMu.RUnlock()

	var eg2 errgroup.Group
	for _, absPath := range absPaths {
		absPath := absPath
		eg2.Go(func() error {
			return gen.convertWithErrorModule(filepath.Dir(absPath), filepath.Base(absPath), false, secondPass)
		})
	}
	return eg2.Wait()
}

func (gen *Generator) generate(dir string) (err error) {
	var eg errgroup.Group
	defer func() {
		if err == nil {
			err = eg.Wait()
		}
	}()

	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return xerrors.WrapKV(err, xerrors.KeyIndir, gen.InputDir)
	}

	// book name -> existence(bool)
	csvBooks := map[string]bool{}
	for _, entry := range dirEntries {
		if entry.IsDir() {
			// scan and generate subdir recursively
			subdir := filepath.Join(dir, entry.Name())
			err = gen.generate(subdir)
			if err != nil {
				return xerrors.WithMessageKV(err, xerrors.KeySubdir, subdir)
			}
			continue
		} else if gen.InputOpt.FollowSymlink && entry.Type() == iofs.ModeSymlink {
			dstPath, err := os.Readlink(filepath.Join(dir, entry.Name()))
			if err != nil {
				return xerrors.WrapKV(err)
			}
			fileInfo, err := os.Stat(dstPath)
			if err != nil {
				return xerrors.WrapKV(err)
			}

			if !fileInfo.IsDir() {
				// is not a directory
				log.Warnf("symlink: %s is not a directory, currently not processed", dstPath)
			}
			err = gen.generate(dstPath)
			if err != nil {
				return xerrors.WithMessageKV(err, xerrors.KeySubdir, dstPath)
			}
			continue
		}

		if strings.HasPrefix(entry.Name(), "~$") {
			// ignore temp file named with prefix "~$"
			continue
		}
		// log.Debugf("generating %s, %s", entry.Name(), filepath.Ext(entry.Name()))
		fmt := format.Ext2Format(filepath.Ext(entry.Name()))
		// check if this workbook format need to be converted
		if !format.FilterInput(fmt, gen.InputOpt.Formats) {
			continue
		}

		if fmt == format.CSV {
			bookName, _, err := fs.ParseCSVFilenamePattern(entry.Name())
			if err != nil {
				return err
			}
			if _, ok := csvBooks[bookName]; ok {
				// NOTE: multiple CSV files construct the same book.
				continue
			}
			csvBooks[bookName] = true
		}

		filename := entry.Name()
		eg.Go(func() error {
			return gen.convertWithErrorModule(dir, filename, true, firstPass)
		})
	}
	return nil
}

func (gen *Generator) addImporter(absPath string, imp importer.Importer) {
	gen.cacheMu.Lock()
	defer gen.cacheMu.Unlock()
	gen.cachedImporters[absPath] = imp
}

func (gen *Generator) getImporter(absPath string) importer.Importer {
	gen.cacheMu.RLock()
	defer gen.cacheMu.RUnlock()
	return gen.cachedImporters[absPath]
}

func (gen *Generator) addBookParser(absPath string, parser *bookParser) {
	gen.cacheMu.Lock()
	defer gen.cacheMu.Unlock()
	gen.cachedBookParsers[absPath] = parser
}

func (gen *Generator) getBookParser(absPath string) *bookParser {
	gen.cacheMu.RLock()
	defer gen.cacheMu.RUnlock()
	return gen.cachedBookParsers[absPath]
}

func (gen *Generator) convertWithErrorModule(dir, filename string, checkProtoFileConflicts bool, pass parsePass) error {
	if err := gen.convert(dir, filename, checkProtoFileConflicts, pass); err != nil {
		return xerrors.WithMessageKV(err, xerrors.KeyModule, xerrors.ModuleProto)
	}
	return nil
}

func (gen *Generator) convert(dir, filename string, checkProtoFileConflicts bool, pass parsePass) (err error) {
	absPath := filepath.Join(dir, filename)
	var imp importer.Importer
	if pass == firstPass {
		parser := confgen.NewSheetParser(TableauProtoPackage, gen.LocationName, book.MetasheetOptions())
		imp, err = importer.New(absPath, importer.Parser(parser), importer.Mode(importer.Protogen))
		if err != nil {
			return xerrors.WrapKV(err, xerrors.KeyBookName, absPath)
		}
		if len(imp.GetSheets()) == 0 {
			return nil
		}
		// cache this new importer
		gen.addImporter(absPath, imp)
	} else {
		imp = gen.getImporter(absPath)
	}

	basename := filepath.Base(imp.Filename())
	relativePath, err := getRelCleanSlashPath(gen.InputDir, dir, basename)
	if err != nil {
		return err
	}
	debugWorkbookName := relativePath
	// rewrite subdir
	rewrittenWorkbookName := fs.RewriteSubdir(relativePath, gen.InputOpt.SubdirRewrites)
	if rewrittenWorkbookName != relativePath {
		debugWorkbookName += " (rewrite: " + rewrittenWorkbookName + ")"
	}

	if pass == firstPass {
		log.Infof("%18s: %s, %d worksheet(s) will be parsed", "analyzing workbook", debugWorkbookName, len(imp.GetSheets()))
	}

	var bp *bookParser
	if pass == firstPass {
		// create a book parser
		bp = newBookParser(imp.BookName(), rewrittenWorkbookName, gen)
		// cache this new bookParser
		gen.addBookParser(absPath, bp)
	} else {
		bp = gen.getBookParser(absPath)
	}

	for _, sheet := range imp.GetSheets() {
		// parse sheet header
		debugSheetName := sheet.Name
		sheetMsgName := sheet.Name
		if sheet.Meta.Alias != "" {
			sheetMsgName = sheet.Meta.Alias
			debugSheetName += " (alias: " + sheet.Meta.Alias + ")"
		}
		mergeHeaderOptions(sheet.Meta, gen.InputOpt.Header)
		ws := &tableaupb.Worksheet{
			Options: &tableaupb.WorksheetOptions{
				Name:          sheet.Name,
				Namerow:       sheet.Meta.Namerow,
				Typerow:       sheet.Meta.Typerow,
				Noterow:       sheet.Meta.Noterow,
				Datarow:       sheet.Meta.Datarow,
				Transpose:     sheet.Meta.Transpose,
				Tags:          "",
				Nameline:      sheet.Meta.Nameline,
				Typeline:      sheet.Meta.Typeline,
				Nested:        sheet.Meta.Nested,
				Sep:           sheet.Meta.Sep,
				Subsep:        sheet.Meta.Subsep,
				Merger:        sheet.Meta.Merger,
				AdjacentKey:   sheet.Meta.AdjacentKey,
				FieldPresence: sheet.Meta.FieldPresence,
				Template:      sheet.Meta.Template,
				Mode:          sheet.Meta.Mode,
				Scatter:       sheet.Meta.Scatter,
				// Loader options:
				OrderedMap: sheet.Meta.OrderedMap,
				Index:      parseIndexes(sheet.Meta.Index),
			},
			Fields: []*tableaupb.Field{},
			Name:   sheetMsgName,
		}

		shHeader := &sheetHeader{
			meta: sheet.Meta,
		}
		// transpose or not
		if sheet.Meta.Transpose {
			for row := 0; row < sheet.MaxRow; row++ {
				nameCol := int(sheet.Meta.Namerow) - 1
				nameCell, err := sheet.Cell(row, nameCol)
				if err != nil {
					return xerrors.WithMessageKV(err, xerrors.KeyBookName, debugWorkbookName, xerrors.KeySheetName, debugSheetName, xerrors.KeyNameCellPos, excel.Postion(row, nameCol))
				}
				shHeader.namerow = append(shHeader.namerow, nameCell)

				typeCol := int(sheet.Meta.Typerow) - 1
				typeCell, err := sheet.Cell(row, typeCol)
				if err != nil {
					return xerrors.WithMessageKV(err, xerrors.KeyBookName, debugWorkbookName, xerrors.KeySheetName, debugSheetName, xerrors.KeyNameCellPos, excel.Postion(row, typeCol))
				}
				shHeader.typerow = append(shHeader.typerow, typeCell)

				noteCol := int(sheet.Meta.Noterow) - 1
				noteCell, err := sheet.Cell(row, noteCol)
				if err != nil {
					return xerrors.WithMessageKV(err, xerrors.KeyBookName, debugWorkbookName, xerrors.KeySheetName, debugSheetName, xerrors.KeyNameCellPos, excel.Postion(row, noteCol))
				}
				shHeader.noterow = append(shHeader.noterow, noteCell)
			}
		} else {
			shHeader.namerow = sheet.GetRow(int(sheet.Meta.Namerow - 1))
			shHeader.typerow = sheet.GetRow(int(sheet.Meta.Typerow - 1))
			shHeader.noterow = sheet.GetRow(int(sheet.Meta.Noterow - 1))
		}
		if pass == firstPass && sheet.Meta.Mode != tableaupb.Mode_MODE_DEFAULT {
			log.Debugf("extract type info from %s", debugSheetName)

			parentFilename := bp.GetProtoFilePath()
			err := gen.extractTypeInfoFromSpecialSheetMode(sheet.Meta.Mode, sheet, ws.Name, parentFilename)
			if err != nil {
				return xerrors.WithMessageKV(err,
					xerrors.KeyBookName, debugWorkbookName,
					xerrors.KeySheetName, debugSheetName)
			}
		} else if pass == secondPass {
			log.Infof("%18s: %s", "parsing worksheet", debugSheetName)

			if sheet.Meta.Mode == tableaupb.Mode_MODE_DEFAULT {
				var parsed bool
				for cursor := 0; cursor < len(shHeader.namerow); cursor++ {
					field := &tableaupb.Field{}
					cursor, parsed, err = bp.parseField(field, shHeader, cursor, "", parseroptions.Nested(sheet.Meta.Nested))
					if err != nil {
						nameCellPos := excel.Postion(int(sheet.Meta.Namerow-1), cursor)
						typeCellPos := excel.Postion(int(sheet.Meta.Typerow-1), cursor)
						if sheet.Meta.Transpose {
							nameCellPos = excel.Postion(cursor, int(sheet.Meta.Namerow-1))
							typeCellPos = excel.Postion(cursor, int(sheet.Meta.Typerow-1))
						}
						return xerrors.WithMessageKV(err,
							xerrors.KeyBookName, debugWorkbookName,
							xerrors.KeySheetName, debugSheetName,
							xerrors.KeyNameCellPos, nameCellPos,
							xerrors.KeyTypeCellPos, typeCellPos,
							xerrors.KeyNameCell, shHeader.getNameCell(cursor),
							xerrors.KeyTypeCell, shHeader.getTypeCell(cursor))
					}
					if parsed {
						ws.Fields = append(ws.Fields, field)
					}
				}
			} else {
				err := gen.parseSpecialSheetMode(sheet.Meta.Mode, ws, sheet)
				if err != nil {
					return err
				}
			}
			// append parsed sheet to workbook
			bp.wb.Worksheets = append(bp.wb.Worksheets, ws)
		}
	}
	// export book
	be := newBookExporter(
		gen.ProtoPackage,
		gen.OutputOpt.FileOptions,
		filepath.Join(gen.OutputDir, gen.OutputOpt.Subdir),
		gen.OutputOpt.FilenameSuffix,
		bp.wb,
		bp.gen,
	)
	if err := be.export(checkProtoFileConflicts); err != nil {
		return xerrors.WithMessageKV(err, xerrors.KeyBookName, debugWorkbookName)
	}

	return nil
}

func (gen *Generator) extractTypeInfoFromSpecialSheetMode(mode tableaupb.Mode, sheet *book.Sheet, typeName, parentFilename string) error {
	// create parser
	sheetOpts := &tableaupb.WorksheetOptions{
		Name:    sheet.Name,
		Namerow: 1,
		Datarow: 2,
	}
	parser := confgen.NewSheetParser(TableauProtoPackage, gen.LocationName, sheetOpts)
	// parse each special sheet mode
	switch mode {
	case tableaupb.Mode_MODE_ENUM_TYPE:
		// add type info
		info := &xproto.TypeInfo{
			FullName:       gen.ProtoPackage + "." + typeName,
			ParentFilename: parentFilename,
			Kind:           types.EnumKind,
		}
		gen.typeInfos.Put(info)
	case tableaupb.Mode_MODE_STRUCT_TYPE:
		desc := &tableaupb.StructDescriptor{}
		if err := parser.Parse(desc, sheet); err != nil {
			return errors.WithMessagef(err, "failed to parse struct type sheet: %s", sheet.Name)
		}
		firstFieldOptionName := ""
		if len(desc.Fields) != 0 {
			firstFieldOptionName = desc.Fields[0].Name
		}
		// add type info
		info := &xproto.TypeInfo{
			FullName:             gen.ProtoPackage + "." + typeName,
			ParentFilename:       parentFilename,
			Kind:                 types.MessageKind,
			FirstFieldOptionName: firstFieldOptionName,
		}
		gen.typeInfos.Put(info)

	case tableaupb.Mode_MODE_UNION_TYPE:
		// add union self type info
		info := &xproto.TypeInfo{
			FullName:             gen.ProtoPackage + "." + typeName,
			ParentFilename:       parentFilename,
			Kind:                 types.MessageKind,
			FirstFieldOptionName: "Type", // NOTE: union's first field name is special!
		}
		gen.typeInfos.Put(info)

		// add union enum type info
		enumInfo := &xproto.TypeInfo{
			FullName:       gen.ProtoPackage + "." + typeName + "." + "Type",
			ParentFilename: parentFilename,
			Kind:           types.EnumKind,
		}
		gen.typeInfos.Put(enumInfo)

		desc := &tableaupb.UnionDescriptor{}
		if err := parser.Parse(desc, sheet); err != nil {
			return errors.WithMessagef(err, "failed to parse union type sheet: %s", sheet.Name)
		}
		// add types nested in union type
		for _, value := range desc.Values {
			firstFieldOptionName := ""
			if len(value.Fields) != 0 {
				// name located at first line of cell
				firstFieldOptionName = book.ExtractFromCell(value.Fields[0], 1)
			}
			info := &xproto.TypeInfo{
				FullName:             gen.ProtoPackage + "." + typeName + "." + value.Name,
				ParentFilename:       parentFilename,
				Kind:                 types.MessageKind,
				FirstFieldOptionName: firstFieldOptionName,
			}
			gen.typeInfos.Put(info)
		}
	default:
		return errors.Errorf("unknown mode: %v", mode)
	}
	return nil
}

func (gen *Generator) parseSpecialSheetMode(mode tableaupb.Mode, ws *tableaupb.Worksheet, sheet *book.Sheet) error {
	// create parser
	sheetOpts := &tableaupb.WorksheetOptions{
		Name:    sheet.Name,
		Namerow: 1,
		Datarow: 2,
	}
	parser := confgen.NewSheetParser(TableauProtoPackage, gen.LocationName, sheetOpts)

	// parse each special sheet mode
	switch mode {
	case tableaupb.Mode_MODE_ENUM_TYPE:
		desc := &tableaupb.EnumDescriptor{}
		if err := parser.Parse(desc, sheet); err != nil {
			return errors.WithMessagef(err, "failed to parse enum type sheet: %s", sheet.Name)
		}
		for i, value := range desc.Values {
			number := int32(i + 1)
			if value.Number != nil {
				number = *value.Number
			}
			field := &tableaupb.Field{
				Number: number,
				Name:   value.Name,
				Alias:  value.Alias,
			}
			ws.Fields = append(ws.Fields, field)
		}
	case tableaupb.Mode_MODE_STRUCT_TYPE:
		desc := &tableaupb.StructDescriptor{}
		if err := parser.Parse(desc, sheet); err != nil {
			return errors.WithMessagef(err, "failed to parse struct type sheet: %s", sheet.Name)
		}
		bp := newBookParser("struct", "", gen)
		shHeader := &sheetHeader{
			meta: &tableaupb.Metasheet{
				Namerow: 1,
				Typerow: 2,
			},
		}
		for _, field := range desc.Fields {
			shHeader.namerow = append(shHeader.namerow, field.Name)
			shHeader.typerow = append(shHeader.typerow, field.Type)
			shHeader.noterow = append(shHeader.noterow, "")
		}
		var parsed bool
		var err error
		for cursor := 0; cursor < len(shHeader.namerow); cursor++ {
			subField := &tableaupb.Field{}
			cursor, parsed, err = bp.parseField(subField, shHeader, cursor, "")
			if err != nil {
				return err
			}
			if parsed {
				ws.Fields = append(ws.Fields, subField)
			}
		}

	case tableaupb.Mode_MODE_UNION_TYPE:
		desc := &tableaupb.UnionDescriptor{}
		if err := parser.Parse(desc, sheet); err != nil {
			return errors.WithMessagef(err, "failed to parse union type sheet: %s", sheet.Name)
		}

		for i, value := range desc.Values {
			number := int32(i + 1)
			if value.Number != nil {
				number = *value.Number
			}
			field := &tableaupb.Field{
				Number: number,
				Name:   value.Name,
				Alias:  value.Alias,
			}
			// create a book parser
			bp := newBookParser("union", "", gen)

			shHeader := &sheetHeader{
				meta: &tableaupb.Metasheet{
					Namerow:  1,
					Typerow:  1,
					Nameline: 1,
					Typeline: 2,
				},
				namerow: value.Fields,
				typerow: value.Fields,
				noterow: value.Fields,
			}
			var parsed bool
			var err error
			for cursor := 0; cursor < len(shHeader.namerow); cursor++ {
				subField := &tableaupb.Field{}
				cursor, parsed, err = bp.parseField(subField, shHeader, cursor, "")
				if err != nil {
					return err
				}
				if parsed {
					field.Fields = append(field.Fields, subField)
				}
			}

			ws.Fields = append(ws.Fields, field)
		}
	default:
		return errors.Errorf("unknown mode: %v", mode)
	}
	return nil
}
