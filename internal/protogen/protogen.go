package protogen

import (
	iofs "io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/confgen"
	"github.com/tableauio/tableau/internal/excel"
	"github.com/tableauio/tableau/internal/importer"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/protogen/parseroptions"
	"github.com/tableauio/tableau/internal/strcase"
	"github.com/tableauio/tableau/internal/types"
	"github.com/tableauio/tableau/internal/x/xfs"
	"github.com/tableauio/tableau/internal/x/xproto"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/proto/tableaupb/internalpb"
	"github.com/tableauio/tableau/xerrors"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
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
	cachedBookParsers map[string]*tableParser      // absolute file path -> bookParser (only for tables currently)
}

func NewGenerator(protoPackage, indir, outdir string, setters ...options.Option) *Generator {
	opts := options.ParseOptions(setters...)
	return NewGeneratorWithOptions(protoPackage, indir, outdir, opts)
}

func NewGeneratorWithOptions(protoPackage, indir, outdir string, opts *options.Options) *Generator {
	gen := &Generator{
		ProtoPackage: protoPackage,
		InputDir:     indir,
		OutputDir:    outdir,
		LocationName: opts.LocationName,
		InputOpt:     opts.Proto.Input,
		OutputOpt:    opts.Proto.Output,

		protofiles: &protoregistry.Files{},
		typeInfos:  xproto.NewTypeInfos(protoPackage),

		cachedImporters:   make(map[string]importer.Importer),
		cachedBookParsers: make(map[string]*tableParser),
	}

	if opts.Proto.Input.MetasheetName != "" {
		book.SetMetasheetName(opts.Proto.Input.MetasheetName)
	}

	for key, val := range opts.Acronyms {
		strcase.ConfigureAcronym(key, val)
	}

	return gen
}

func (gen *Generator) preprocess(delExisted bool) error {
	// parse custom imported proto files
	protofiles, err := xproto.ParseProtos(
		gen.InputOpt.ProtoPaths,
		gen.InputOpt.ProtoFiles...)
	if err != nil {
		return err
	}
	gen.protofiles = protofiles
	gen.typeInfos = xproto.GetAllTypeInfo(protofiles, gen.ProtoPackage)

	outputProtoDir := filepath.Join(gen.OutputDir, gen.OutputOpt.Subdir)
	if err := prepareOutdir(outputProtoDir, gen.InputOpt.ProtoFiles, delExisted); err != nil {
		return err
	}
	return nil
}

func (gen *Generator) Generate(relWorkbookPaths ...string) error {
	if len(relWorkbookPaths) == 0 {
		return gen.GenAll()
	}
	return gen.GenWorkbook(relWorkbookPaths...)
}

func (gen *Generator) GenAll() error {
	if err := gen.preprocess(true); err != nil {
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
	if err := gen.preprocess(false); err != nil {
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
	for absPath := range gen.cachedImporters {
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
				return xerrors.WrapKV(err, xerrors.KeySubdir, subdir)
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
				return xerrors.WrapKV(err, xerrors.KeySubdir, dstPath)
			}
			continue
		}

		if strings.HasPrefix(entry.Name(), "~$") {
			// ignore temp file named with prefix "~$"
			continue
		}
		// log.Debugf("generating %s, %s", entry.Name(), filepath.Ext(entry.Name()))
		fmt := format.GetFormat(entry.Name())
		// check if this workbook format need to be converted
		if !format.FilterInput(fmt, gen.InputOpt.Formats) {
			continue
		}

		if fmt == format.CSV {
			bookName, _, err := xfs.ParseCSVFilenamePattern(entry.Name())
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

func (gen *Generator) addBookParser(absPath string, parser *tableParser) {
	gen.cacheMu.Lock()
	defer gen.cacheMu.Unlock()
	gen.cachedBookParsers[absPath] = parser
}

func (gen *Generator) getBookParser(absPath string) *tableParser {
	gen.cacheMu.RLock()
	defer gen.cacheMu.RUnlock()
	return gen.cachedBookParsers[absPath]
}

func (gen *Generator) convertWithErrorModule(dir, filename string, checkProtoFileConflicts bool, pass parsePass) error {
	fmt := format.GetFormat(filename)
	if format.IsInputDocumentFormat(fmt) {
		if err := gen.convertDocument(dir, filename, checkProtoFileConflicts, pass); err != nil {
			return xerrors.WrapKV(err, xerrors.KeyModule, xerrors.ModuleProto)
		}
		return nil
	}
	if err := gen.convertTable(dir, filename, checkProtoFileConflicts, pass); err != nil {
		return xerrors.WrapKV(err, xerrors.KeyModule, xerrors.ModuleProto)
	}
	return nil
}

func (gen *Generator) convertDocument(dir, filename string, checkProtoFileConflicts bool, pass parsePass) (err error) {
	if pass == secondPass {
		// NOTE: currently, document do not support two-pass parsing, so just return nil.
		return nil
	}
	absPath := filepath.Join(dir, filename)
	parser := confgen.NewSheetParser(xproto.InternalProtoPackage, gen.LocationName, book.MetasheetOptions())
	imp, err := importer.New(absPath, importer.Parser(parser), importer.Mode(importer.Protogen))
	if err != nil {
		return xerrors.WrapKV(err, xerrors.KeyBookName, absPath)
	}
	if len(imp.GetSheets()) == 0 {
		return nil
	}
	basename := filepath.Base(imp.Filename())
	relativePath, err := getRelCleanSlashPath(gen.InputDir, dir, basename)
	if err != nil {
		return err
	}
	debugBookName := relativePath
	// rewrite subdir
	rewrittenBookName := xfs.RewriteSubdir(relativePath, gen.InputOpt.SubdirRewrites)
	if rewrittenBookName != relativePath {
		debugBookName += " (rewrite: " + rewrittenBookName + ")"
	}

	log.Infof("%15s: %s, %d worksheet(s) will be parsed", "analyzing book", debugBookName, len(imp.GetSheets()))

	// create a book parser
	bookName := imp.BookName()
	bookOpts := imp.GetBookOptions()
	alias := bookOpts.GetAlias()
	if alias != "" {
		debugBookName += " (alias: " + alias + ")"
	}
	bp := newDocumentParser(bookName, alias, rewrittenBookName, gen)
	for _, sheet := range imp.GetSheets() {
		// parse sheet options
		ws := sheet.ToWorkseet()
		debugSheetName := sheet.GetDebugName()
		log.Infof("%15s: %s", "parsing sheet", debugSheetName)

		// log.Debugf("dump document:\n%s", sheet.String())
		if len(sheet.Document.Children) != 1 {
			return xerrors.Errorf("document should have and only have one child (map node), sheet: %s", sheet.Name)
		}
		// get the first child (map node) in document
		child := sheet.Document.Children[0]
		var parsed bool
		for _, node := range child.Children {
			field := &internalpb.Field{}
			parsed, err = bp.parseField(field, node)
			if err != nil {
				return xerrors.WrapKV(err,
					xerrors.KeyBookName, debugBookName,
					xerrors.KeySheetName, debugSheetName,
				)
			}
			if parsed {
				ws.Fields = append(ws.Fields, field)
			}
		}
		// append parsed sheet to workbook
		bp.wb.Worksheets = append(bp.wb.Worksheets, ws)
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
		return xerrors.WrapKV(err, xerrors.KeyBookName, debugBookName)
	}
	return nil
}

func (gen *Generator) convertTable(dir, filename string, checkProtoFileConflicts bool, pass parsePass) (err error) {
	absPath := filepath.Join(dir, filename)
	var imp importer.Importer
	if pass == firstPass {
		parser := confgen.NewSheetParser(xproto.InternalProtoPackage, gen.LocationName, book.MetasheetOptions())
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
	debugBookName := relativePath
	// rewrite subdir
	rewrittenBookName := xfs.RewriteSubdir(relativePath, gen.InputOpt.SubdirRewrites)
	if rewrittenBookName != relativePath {
		debugBookName += " (rewrite: " + rewrittenBookName + ")"
	}

	if pass == firstPass {
		log.Infof("%15s: %s, %d worksheet(s) will be parsed", "analyzing book", debugBookName, len(imp.GetSheets()))
	}
	bookOpts := imp.GetBookOptions()
	var bp *tableParser
	if pass == firstPass {
		// create a book parser
		bookName := imp.BookName()
		alias := bookOpts.GetAlias()
		if alias != "" {
			debugBookName += " (alias: " + alias + ")"
		}
		bp = newTableParser(bookName, alias, rewrittenBookName, gen)
		// cache this new tableParser
		gen.addBookParser(absPath, bp)
	} else {
		bp = gen.getBookParser(absPath)
	}

	for _, sheet := range imp.GetSheets() {
		// parse sheet header
		ws := sheet.ToWorkseet()
		debugSheetName := sheet.GetDebugName()
		if pass == firstPass {
			log.Infof("%15s: %s", "parsing sheet", debugSheetName)
		}

		tableHeader := newTableHeader(ws.Options, bookOpts, gen.InputOpt.Header)
		// transpose or not
		if ws.Options.Transpose {
			for row := 0; row < sheet.Table.MaxRow; row++ {
				nameCol := tableHeader.NameRow - 1
				nameCell, err := sheet.Table.Cell(row, nameCol)
				if err != nil {
					return xerrors.WrapKV(err, xerrors.KeyBookName, debugBookName, xerrors.KeySheetName, debugSheetName, xerrors.KeyNameCellPos, excel.Postion(row, nameCol))
				}
				tableHeader.nameRowData = append(tableHeader.nameRowData, nameCell)

				typeCol := tableHeader.TypeRow - 1
				typeCell, err := sheet.Table.Cell(row, typeCol)
				if err != nil {
					return xerrors.WrapKV(err, xerrors.KeyBookName, debugBookName, xerrors.KeySheetName, debugSheetName, xerrors.KeyNameCellPos, excel.Postion(row, typeCol))
				}
				tableHeader.typeRowData = append(tableHeader.typeRowData, typeCell)

				noteCol := tableHeader.NoteRow - 1
				noteCell, err := sheet.Table.Cell(row, noteCol)
				if err != nil {
					return xerrors.WrapKV(err, xerrors.KeyBookName, debugBookName, xerrors.KeySheetName, debugSheetName, xerrors.KeyNameCellPos, excel.Postion(row, noteCol))
				}
				tableHeader.noteRowData = append(tableHeader.noteRowData, noteCell)
			}
		} else {
			tableHeader.nameRowData = sheet.Table.GetRow(tableHeader.NameRow - 1)
			tableHeader.typeRowData = sheet.Table.GetRow(tableHeader.TypeRow - 1)
			tableHeader.noteRowData = sheet.Table.GetRow(tableHeader.NoteRow - 1)
		}

		// Two-pass flow:
		// 	1. first pass: extract type info from special sheet mode (none default mode)
		// 	2. second pass: parse sheet schema
		if pass == firstPass && ws.Options.Mode != tableaupb.Mode_MODE_DEFAULT {
			log.Debugf("first pass: extract type info from %s", debugSheetName)

			parentFilename := bp.GetProtoFilePath()
			err := gen.extractTypeInfoFromSpecialSheetMode(ws.Options.Mode, sheet, ws.Name, parentFilename)
			if err != nil {
				return xerrors.WrapKV(err,
					xerrors.KeyBookName, debugBookName,
					xerrors.KeySheetName, debugSheetName)
			}
		} else if pass == secondPass {
			log.Debugf("second pass: parse sheet schema from %s", debugSheetName)
			if ws.Options.Mode == tableaupb.Mode_MODE_DEFAULT {
				var parsed bool
				for cursor := 0; cursor < len(tableHeader.nameRowData); cursor++ {
					field := &internalpb.Field{}
					cursor, parsed, err = bp.parseField(field, tableHeader, cursor, "", parseroptions.Nested(ws.Options.Nested))
					if err != nil {
						return wrapDebugErr(err, debugBookName, debugSheetName, tableHeader, cursor)
					}
					if parsed {
						ws.Fields = append(ws.Fields, field)
					}
				}
				// append parsed sheet to workbook
				bp.wb.Worksheets = append(bp.wb.Worksheets, ws)
			} else {
				worksheets, err := gen.parseSpecialSheetMode(ws.Options.Mode, ws, sheet, debugBookName, debugSheetName)
				if err != nil {
					return err
				}
				// append parsed sheets to workbook
				bp.wb.Worksheets = append(bp.wb.Worksheets, worksheets...)
			}
		}
	}

	if pass == secondPass {
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
			return xerrors.WrapKV(err, xerrors.KeyBookName, debugBookName)
		}
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
	parser := confgen.NewSheetParser(xproto.InternalProtoPackage, gen.LocationName, sheetOpts)
	// parse each special sheet mode
	switch mode {
	case tableaupb.Mode_MODE_ENUM_TYPE:
		// add type info
		info := &xproto.TypeInfo{
			FullName:       protoreflect.FullName(gen.ProtoPackage + "." + typeName),
			ParentFilename: parentFilename,
			Kind:           types.EnumKind,
		}
		gen.typeInfos.Put(info)
	case tableaupb.Mode_MODE_ENUM_TYPE_MULTI:
		for row := 0; row <= sheet.Table.MaxRow; row++ {
			cols := sheet.Table.GetRow(row)
			if isEnumTypeBlockHeader(cols) {
				if row < 1 {
					continue
				}
				typeRow := sheet.Table.GetRow(row - 1)
				typeName, _, err := extractSheetBlockTypeRow(typeRow)
				if err != nil {
					return xerrors.Wrapf(err, "failed to parse enum type block, sheet: %s, row: %d", sheet.Name, row)
				}
				// add type info
				info := &xproto.TypeInfo{
					FullName:       protoreflect.FullName(gen.ProtoPackage + "." + typeName),
					ParentFilename: parentFilename,
					Kind:           types.EnumKind,
				}
				gen.typeInfos.Put(info)
			}
		}
	case tableaupb.Mode_MODE_STRUCT_TYPE:
		if err := extractStructTypeInfo(sheet, typeName, parentFilename, parser, gen); err != nil {
			return err
		}
	case tableaupb.Mode_MODE_STRUCT_TYPE_MULTI:
		for row := 0; row <= sheet.Table.MaxRow; row++ {
			cols := sheet.Table.GetRow(row)
			if isStructTypeBlockHeader(cols) {
				if row < 1 {
					continue
				}
				typeRow := sheet.Table.GetRow(row - 1)
				typeName, _, err := extractSheetBlockTypeRow(typeRow)
				if err != nil {
					return xerrors.Wrapf(err, "failed to parse struct type block at row: %d, sheet: %s", row, sheet.Name)
				}
				blockBeginRow := row
				block, blockEndRow := sheet.Table.ExtractBlock(blockBeginRow)
				row = blockEndRow // skip row to next block
				subSheet := book.NewTableSheet(sheet.Name, block)
				if err := extractStructTypeInfo(subSheet, typeName, parentFilename, parser, gen); err != nil {
					return err
				}
			}
		}
	case tableaupb.Mode_MODE_UNION_TYPE:
		if err := extractUnionTypeInfo(sheet, typeName, parentFilename, parser, gen); err != nil {
			return err
		}
	case tableaupb.Mode_MODE_UNION_TYPE_MULTI:
		for row := 0; row <= sheet.Table.MaxRow; row++ {
			cols := sheet.Table.GetRow(row)
			if isUnionTypeBlockHeader(cols) {
				if row < 1 {
					continue
				}
				typeRow := sheet.Table.GetRow(row - 1)
				typeName, _, err := extractSheetBlockTypeRow(typeRow)
				if err != nil {
					return xerrors.Wrapf(err, "failed to parse union type block, sheet: %s, row: %d", sheet.Name, row)
				}
				blockBeginRow := row
				block, blockEndRow := sheet.Table.ExtractBlock(blockBeginRow)
				row = blockEndRow // skip row to next block
				subSheet := book.NewTableSheet(sheet.Name, block)
				if err := extractUnionTypeInfo(subSheet, typeName, parentFilename, parser, gen); err != nil {
					return err
				}
			}
		}
	default:
		return xerrors.Errorf("unknown mode: %v", mode)
	}
	return nil
}

func (gen *Generator) parseSpecialSheetMode(mode tableaupb.Mode, ws *internalpb.Worksheet, sheet *book.Sheet, debugBookName, debugSheetName string) ([]*internalpb.Worksheet, error) {
	// create parser
	sheetOpts := &tableaupb.WorksheetOptions{
		Name:    sheet.Name,
		Namerow: 1,
		Datarow: 2,
	}
	parser := confgen.NewSheetParser(xproto.InternalProtoPackage, gen.LocationName, sheetOpts)

	// parse each special sheet mode
	switch mode {
	case tableaupb.Mode_MODE_ENUM_TYPE:
		if err := parseEnumType(ws, sheet, parser, gen.OutputOpt.EnumValueWithPrefix); err != nil {
			return nil, err
		}
		return []*internalpb.Worksheet{ws}, nil
	case tableaupb.Mode_MODE_ENUM_TYPE_MULTI:
		var worksheets []*internalpb.Worksheet
		for row := 0; row <= sheet.Table.MaxRow; row++ {
			cols := sheet.Table.GetRow(row)
			if isEnumTypeBlockHeader(cols) {
				if row < 1 {
					continue
				}
				blockBeginRow := row
				typeRow := sheet.Table.GetRow(row - 1)
				var err error
				subWs := proto.Clone(ws).(*internalpb.Worksheet)
				subWs.Name, subWs.Note, err = extractSheetBlockTypeRow(typeRow)
				if err != nil {
					return nil, xerrors.Wrapf(err, "failed to extract enum type block at row: %d, sheet: %s", row, sheet.Name)
				}
				block, blockEndRow := sheet.Table.ExtractBlock(blockBeginRow)
				row = blockEndRow // skip row to next block
				subSheet := book.NewTableSheet(subWs.Name, block)
				if err := parseEnumType(subWs, subSheet, parser, gen.OutputOpt.EnumValueWithPrefix); err != nil {
					return nil, err
				}
				worksheets = append(worksheets, subWs)
			}
		}
		return worksheets, nil
	case tableaupb.Mode_MODE_STRUCT_TYPE:
		if err := parseStructType(ws, sheet, parser, gen, debugBookName, debugSheetName); err != nil {
			return nil, err
		}
		return []*internalpb.Worksheet{ws}, nil
	case tableaupb.Mode_MODE_STRUCT_TYPE_MULTI:
		var worksheets []*internalpb.Worksheet
		for row := 0; row <= sheet.Table.MaxRow; row++ {
			cols := sheet.Table.GetRow(row)
			if isStructTypeBlockHeader(cols) {
				if row < 1 {
					continue
				}
				blockBeginRow := row
				typeRow := sheet.Table.GetRow(row - 1)
				var err error
				subWs := proto.Clone(ws).(*internalpb.Worksheet)
				subWs.Name, subWs.Note, err = extractSheetBlockTypeRow(typeRow)
				if err != nil {
					return nil, xerrors.Wrapf(err, "failed to extract struct type block at row: %d, sheet: %s", row, sheet.Name)
				}
				block, blockEndRow := sheet.Table.ExtractBlock(blockBeginRow)
				row = blockEndRow // skip row to next block
				subSheet := book.NewTableSheet(subWs.Name, block)
				if err := parseStructType(subWs, subSheet, parser, gen, debugBookName, debugSheetName); err != nil {
					return nil, err
				}
				worksheets = append(worksheets, subWs)
			}
		}
		return worksheets, nil
	case tableaupb.Mode_MODE_UNION_TYPE:
		if err := parseUnionType(ws, sheet, parser, gen, debugBookName, debugSheetName); err != nil {
			return nil, err
		}
		return []*internalpb.Worksheet{ws}, nil
	case tableaupb.Mode_MODE_UNION_TYPE_MULTI:
		var worksheets []*internalpb.Worksheet
		for row := 0; row <= sheet.Table.MaxRow; row++ {
			cols := sheet.Table.GetRow(row)
			if isUnionTypeBlockHeader(cols) {
				if row < 1 {
					continue
				}
				blockBeginRow := row
				typeRow := sheet.Table.GetRow(row - 1)
				var err error
				subWs := proto.Clone(ws).(*internalpb.Worksheet)
				subWs.Name, subWs.Note, err = extractSheetBlockTypeRow(typeRow)
				if err != nil {
					return nil, xerrors.Wrapf(err, "failed to extract union type block at row: %d, sheet: %s", row, sheet.Name)
				}
				block, blockEndRow := sheet.Table.ExtractBlock(blockBeginRow)
				row = blockEndRow // skip row to next block
				subSheet := book.NewTableSheet(subWs.Name, block)
				if err := parseUnionType(subWs, subSheet, parser, gen, debugBookName, debugSheetName); err != nil {
					return nil, err
				}
				worksheets = append(worksheets, subWs)
			}
		}
		return worksheets, nil
	default:
		return nil, xerrors.Errorf("unknown mode: %v", mode)
	}
}
