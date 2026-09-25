package protogen

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/confgen"
	"github.com/tableauio/tableau/internal/importer"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/importer/book/tableparser"
	"github.com/tableauio/tableau/internal/importer/metasheet"
	"github.com/tableauio/tableau/internal/strcase"
	"github.com/tableauio/tableau/internal/types"
	"github.com/tableauio/tableau/internal/x/xerrors"
	"github.com/tableauio/tableau/internal/x/xfs"
	"github.com/tableauio/tableau/internal/x/xproto"
	"github.com/tableauio/tableau/internal/x/xproto/protoc"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/proto/tableaupb/internalpb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
)

type Generator struct {
	ctx          context.Context
	ProtoPackage string // protobuf package name.
	InputDir     string // input dir of workbooks.
	OutputDir    string // output dir of generated protoconf files.

	LocationName  string                     // TZ location name.
	InputOpt      *options.ProtoInputOption  // Input settings.
	OutputOpt     *options.ProtoOutputOption // Output settings.
	ErrorLimitOpt *options.ErrorLimitOption  // error collection limits.

	ProtoRegistryFiles *protoregistry.Files
	ProtoRegistryTypes *dynamicpb.Types

	// internal
	typeInfos *xproto.TypeInfos  // predefined type infos
	collector *xerrors.Collector // concurrent error collector shared across the generator.

	// used in advanced mode or when preserveFieldNumbers is set to true
	registryWithGeneratedOnce       sync.Once
	protoRegistryFilesWithGenerated *protoregistry.Files

	cacheMu         sync.RWMutex                 // guard fields below
	cachedImporters map[string]importer.Importer // absolute file path -> importer

	runMu  sync.Mutex // public generation calls run one at a time
	output *protoOutput
}

func NewGenerator(protoPackage, indir, outdir string, setters ...options.Option) *Generator {
	opts := options.ParseOptions(setters...)
	return NewGeneratorWithOptions(protoPackage, indir, outdir, opts)
}

func NewGeneratorWithOptions(protoPackage, indir, outdir string, opts *options.Options) *Generator {
	ctx := context.Background()
	ctx = strcase.NewContext(ctx, strcase.New(opts.Acronyms))
	ctx = metasheet.NewContext(ctx, &metasheet.Metasheet{Name: opts.Proto.Input.MetasheetName})

	errorLimit := opts.ErrorLimit
	if errorLimit == nil {
		errorLimit = &options.ErrorLimitOption{
			MaxErrors:         options.DefaultMaxErrors,
			MaxErrorsPerBook:  options.DefaultMaxErrorsPerBook,
			MaxErrorsPerSheet: options.DefaultMaxErrorsPerSheet,
		}
	}

	gen := &Generator{
		ProtoPackage:  protoPackage,
		InputDir:      indir,
		OutputDir:     outdir,
		LocationName:  opts.LocationName,
		InputOpt:      opts.Proto.Input,
		OutputOpt:     opts.Proto.Output,
		ErrorLimitOpt: errorLimit,
		ctx:           ctx,
		typeInfos:     xproto.NewTypeInfos(protoPackage),
		collector:     xerrors.NewCollector(errorLimit.MaxErrors),

		cachedImporters: make(map[string]importer.Importer),
		output:          newProtoOutput(filepath.Join(outdir, opts.Proto.Output.Subdir), nil),
	}
	registryFiles, err := gen.parseProtoRegistryFiles(false)
	if err != nil {
		panic(err)
	}
	gen.ProtoRegistryFiles = registryFiles
	gen.ProtoRegistryTypes = dynamicpb.NewTypes(registryFiles)
	// NOTE: protoRegistryFilesWithGenerated is lazily computed by
	// getProtoRegistryFilesIncludingGenerated() on first use.
	return gen
}

// resetRunState clears state created by the previous generation run.
func (gen *Generator) resetRunState() error {
	protectedPaths, err := resolveImportedProtoPaths(gen.InputOpt.ProtoFiles)
	if err != nil {
		return err
	}
	gen.output = newProtoOutput(filepath.Join(gen.OutputDir, gen.OutputOpt.Subdir), protectedPaths)
	gen.collector = xerrors.NewCollector(gen.ErrorLimitOpt.MaxErrors)
	gen.registryWithGeneratedOnce = sync.Once{}
	gen.protoRegistryFilesWithGenerated = nil
	gen.cacheMu.Lock()
	gen.cachedImporters = make(map[string]importer.Importer)
	gen.cacheMu.Unlock()
	return nil
}

// getProtoRegistryFilesIncludingGenerated returns a registry including both the
// imported and previously generated protos, computing and caching it on
// first use.
func (gen *Generator) getProtoRegistryFilesIncludingGenerated() *protoregistry.Files {
	if gen.protoRegistryFilesWithGenerated != nil {
		return gen.protoRegistryFilesWithGenerated
	}
	gen.registryWithGeneratedOnce.Do(func() {
		files, err := gen.parseProtoRegistryFiles(true)
		if err != nil {
			panic(err)
		}
		gen.protoRegistryFilesWithGenerated = files
	})
	return gen.protoRegistryFilesWithGenerated
}

func (gen *Generator) parseProtoRegistryFiles(useGeneratedProtos bool) (*protoregistry.Files, error) {
	outdir := filepath.Join(gen.OutputDir, gen.OutputOpt.Subdir)
	var protoFiles []string
	protoFiles = append(protoFiles, gen.InputOpt.ProtoFiles...)
	if useGeneratedProtos {
		protoFiles = append(protoFiles, xfs.CleanSlashPath(filepath.Join(outdir, "*.proto")))
	}
	// parse custom imported proto files
	return protoc.NewFiles(
		gen.InputOpt.ProtoPaths,
		protoFiles)
}

// preprocess loads external declarations and prepares the output directory.
// Generated protos are included only for advanced selected generation.
func (gen *Generator) preprocess(includeGeneratedProtos bool) error {
	outdir := filepath.Join(gen.OutputDir, gen.OutputOpt.Subdir)
	protoRegistryFiles := gen.ProtoRegistryFiles
	if includeGeneratedProtos {
		// Advanced mode parses only the selected workbooks. Existing generated
		// protos provide the types from every other workbook.
		protoRegistryFiles = gen.getProtoRegistryFilesIncludingGenerated()
	} else if gen.OutputOpt.PreserveFieldNumbers {
		// Keep the previous schema available for field-number preservation,
		// without treating its types as inputs to this run.
		_ = gen.getProtoRegistryFilesIncludingGenerated()
	}
	gen.typeInfos = xproto.GetAllTypeInfo(protoRegistryFiles, gen.ProtoPackage)
	return ensureOutputDir(outdir)
}

// Generate generates proto files for the specified workbooks. If no workbook paths are provided,
// it generates proto files for all workbooks found in the input directory.
func (gen *Generator) Generate(relWorkbookPaths ...string) error {
	if len(relWorkbookPaths) == 0 {
		return gen.GenAll()
	}
	return gen.GenWorkbook(relWorkbookPaths...)
}

// GenAll generates proto files for every input workbook.
func (gen *Generator) GenAll() error {
	gen.runMu.Lock()
	defer gen.runMu.Unlock()
	if err := gen.resetRunState(); err != nil {
		return err
	}
	if err := gen.preprocess(false); err != nil {
		return err
	}
	if err := gen.parseAllInFirstPass(); err != nil {
		return err
	}
	if err := gen.output.createStagingDir(); err != nil {
		return err
	}
	defer gen.output.removeStagingDir()
	if err := gen.parseAllInSecondPass(); err != nil {
		return err
	}
	// Generation errors leave prior outputs untouched. GenAll alone owns the
	// top-level output directory, so it also removes stale files on commit.
	return gen.output.publishAll()
}

// GenWorkbook generates proto files for the specified input workbooks.
func (gen *Generator) GenWorkbook(relWorkbookPaths ...string) error {
	gen.runMu.Lock()
	defer gen.runMu.Unlock()
	if err := gen.resetRunState(); err != nil {
		return err
	}

	// Preprocess and first-pass parsing establish the declarations needed to
	// parse the selected workbooks in the second pass.
	switch gen.InputOpt.FirstPassMode {
	case options.FirstPassModeNormal:
		// Parse all input workbooks so declarations come from source files.
		if err := gen.preprocess(false); err != nil {
			return err
		}
		if err := gen.parseAllInFirstPass(); err != nil {
			return err
		}
	case options.FirstPassModeAdvanced:
		// Reuse generated declarations for workbooks outside this selection.
		if err := gen.preprocess(true); err != nil {
			return err
		}
		if err := gen.parseWorkbooksInFirstPass(relWorkbookPaths...); err != nil {
			return err
		}
	default:
		// Build declarations only from the selected input workbooks.
		if err := gen.preprocess(false); err != nil {
			return err
		}
		if err := gen.parseWorkbooksInFirstPass(relWorkbookPaths...); err != nil {
			return err
		}
	}

	if err := gen.output.createStagingDir(); err != nil {
		return err
	}
	defer gen.output.removeStagingDir()
	if err := gen.parseWorkbooksInSecondPass(relWorkbookPaths...); err != nil {
		return err
	}
	// Other workbooks' outputs remain valid when generating a selection.
	return gen.output.publishSelected()
}

// parseAllInFirstPass discovers the declarations from every configured input workbook.
func (gen *Generator) parseAllInFirstPass() error {
	log.Infof("%15s: parsing all books", "first-pass")
	if len(gen.InputOpt.Subdirs) == 0 {
		return gen.parseDirInFirstPass(gen.InputDir)
	}
	for _, subdir := range gen.InputOpt.Subdirs {
		dir := filepath.Join(gen.InputDir, subdir)
		if err := gen.parseDirInFirstPass(dir); err != nil {
			return err
		}
	}
	return nil
}

// parseWorkbooksInFirstPass discovers declarations from the specified workbooks.
func (gen *Generator) parseWorkbooksInFirstPass(relWorkbookPaths ...string) error {
	g := gen.collector.NewGroup(context.Background())
	for _, relWorkbookPath := range relWorkbookPaths {
		absPath := filepath.Join(gen.InputDir, relWorkbookPath)
		g.Go(func(ctx context.Context) error {
			return gen.convertWithErrorModule(filepath.Dir(absPath), filepath.Base(absPath), firstPass)
		})
	}
	return g.Wait()
}

// parseDirInFirstPass discovers declarations from workbooks in dir and its subdirectories.
func (gen *Generator) parseDirInFirstPass(dir string) (err error) {
	g := gen.collector.NewGroup(context.Background())
	defer func() {
		if err == nil {
			err = g.Wait()
		}
	}()

	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return xerrors.WrapKV(err, xerrors.KeyIndir, gen.InputDir)
	}

	// A CSV workbook may consist of multiple files; parse it once.
	csvBooks := map[string]bool{}
	for _, entry := range dirEntries {
		if entry.IsDir() {
			// Scan nested input directories recursively.
			subdir := filepath.Join(dir, entry.Name())
			err = gen.parseDirInFirstPass(subdir)
			if err != nil {
				return xerrors.WrapKV(err, xerrors.KeySubdir, subdir)
			}
			continue
		}
		if gen.InputOpt.FollowSymlink && entry.Type() == fs.ModeSymlink {
			dstPath, err := os.Readlink(filepath.Join(dir, entry.Name()))
			if err != nil {
				return xerrors.WrapKV(err)
			}
			fileInfo, err := os.Stat(dstPath)
			if err != nil {
				return xerrors.WrapKV(err)
			}
			if !fileInfo.IsDir() {
				log.Warnf("symlink: %s is not a directory, skipping", dstPath)
				continue
			}
			err = gen.parseDirInFirstPass(dstPath)
			if err != nil {
				return xerrors.WrapKV(err, xerrors.KeySubdir, dstPath)
			}
			continue
		}

		if strings.HasPrefix(entry.Name(), "~$") {
			// Skip Office temporary files.
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
		g.Go(func(ctx context.Context) error {
			return gen.convertWithErrorModule(dir, filename, firstPass)
		})
	}
	return nil
}

// parseAllInSecondPass parses every workbook discovered during the first pass and
// writes its proto files.
func (gen *Generator) parseAllInSecondPass() error {
	gen.cacheMu.RLock()
	absPaths := []string{}
	for absPath := range gen.cachedImporters {
		absPaths = append(absPaths, absPath)
	}
	gen.cacheMu.RUnlock()

	g := gen.collector.NewGroup(context.Background())
	for _, absPath := range absPaths {
		g.Go(func(ctx context.Context) error {
			return gen.convertWithErrorModule(filepath.Dir(absPath), filepath.Base(absPath), secondPass)
		})
	}
	return g.Wait()
}

// parseWorkbooksInSecondPass parses the specified workbooks and writes their proto files.
func (gen *Generator) parseWorkbooksInSecondPass(relWorkbookPaths ...string) error {
	g := gen.collector.NewGroup(context.Background())
	for _, relWorkbookPath := range relWorkbookPaths {
		absPath := filepath.Join(gen.InputDir, relWorkbookPath)
		g.Go(func(ctx context.Context) error {
			return gen.convertWithErrorModule(filepath.Dir(absPath), filepath.Base(absPath), secondPass)
		})
	}
	return g.Wait()
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

func (gen *Generator) convertWithErrorModule(dir, filename string, pass parsePass) error {
	fmt := format.GetFormat(filename)
	if format.IsInputDocumentFormat(fmt) {
		if err := gen.convertDocument(dir, filename, pass); err != nil {
			return xerrors.WrapKV(err, xerrors.KeyModule, xerrors.ModuleProto)
		}
		return nil
	}
	if err := gen.convertTable(dir, filename, pass); err != nil {
		return xerrors.WrapKV(err, xerrors.KeyModule, xerrors.ModuleProto)
	}
	return nil
}

func (gen *Generator) convertDocument(dir, filename string, pass parsePass) (err error) {
	if pass == secondPass {
		// NOTE: currently, document do not support two-pass parsing, so just return nil.
		return nil
	}
	absPath := filepath.Join(dir, filename)
	parser := confgen.NewSheetParser(gen.ctx, xproto.InternalProtoPackage, gen.LocationName, book.MetasheetOptions(gen.ctx))
	imp, err := importer.New(gen.ctx, absPath, importer.Parser(parser), importer.Mode(importer.Protogen))
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

	log.Infof("%15s: %s, %d sheet(s) will be parsed", "analyzing book", debugBookName, len(imp.GetSheets()))

	// create a book parser
	bookName := imp.BookName()
	bookOpts := imp.GetBookOptions()
	alias := bookOpts.GetAlias()
	if alias != "" {
		debugBookName += " (alias: " + alias + ")"
	}
	bp := newDocumentParser(bookName, alias, rewrittenBookName, gen)
	bookCollector := gen.collector.NewChild(gen.ErrorLimitOpt.MaxErrorsPerBook,
		xerrors.KeyModule, xerrors.ModuleProto,
		xerrors.KeyBookName, debugBookName)
	for _, sheet := range imp.GetSheets() {
		sheetErr := gen.convertDocumentSheet(bp, bookCollector, sheet, debugBookName)
		if err := bookCollector.Collect(sheetErr); err != nil {
			return err
		}
	}
	if bookCollector.HasErrors() {
		return bookCollector.Join()
	}
	// export book
	be := newBookExporter(
		gen.ProtoPackage,
		gen.OutputOpt.Edition,
		gen.OutputOpt.FileOptions,
		filepath.Join(gen.OutputDir, gen.OutputOpt.Subdir),
		gen.OutputOpt.FilenameSuffix,
		bp.wb,
		bp.gen,
	)
	if err := be.export(); err != nil {
		return xerrors.WrapKV(err, xerrors.KeyBookName, debugBookName)
	}
	return nil
}

func (gen *Generator) convertTable(dir, filename string, pass parsePass) (err error) {
	absPath := filepath.Join(dir, filename)
	imp := gen.getImporter(absPath)
	if imp == nil {
		parser := confgen.NewSheetParser(gen.ctx, xproto.InternalProtoPackage, gen.LocationName, book.MetasheetOptions(gen.ctx))
		imp, err = importer.New(gen.ctx, absPath, importer.Parser(parser), importer.Mode(importer.Protogen))
		if err != nil {
			return xerrors.WrapKV(err, xerrors.KeyBookName, absPath)
		}
		if len(imp.GetSheets()) == 0 {
			return nil
		}
		// cache this new importer
		gen.addImporter(absPath, imp)
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
	// alias
	bookOpts := imp.GetBookOptions()
	bookName := imp.BookName()
	alias := bookOpts.GetAlias()
	if alias != "" {
		debugBookName += " (alias: " + alias + ")"
	}
	if pass == secondPass {
		log.Infof("%15s: %s, %d sheet(s) will be parsed", "analyzing book", debugBookName, len(imp.GetSheets()))
	}
	// create a book parser
	bp := newTableParser(bookName, alias, rewrittenBookName, gen)
	bookCollector := gen.collector.NewChild(gen.ErrorLimitOpt.MaxErrorsPerBook,
		xerrors.KeyModule, xerrors.ModuleProto,
		xerrors.KeyBookName, debugBookName)
	for _, sheet := range imp.GetSheets() {
		sheetErr := gen.convertTableSheet(bp, bookCollector, sheet, bookOpts, debugBookName, pass)
		if err := bookCollector.Collect(sheetErr); err != nil {
			return err
		}
	}
	if bookCollector.HasErrors() {
		return bookCollector.Join()
	}

	if pass == secondPass {
		// export book
		be := newBookExporter(
			gen.ProtoPackage,
			gen.OutputOpt.Edition,
			gen.OutputOpt.FileOptions,
			filepath.Join(gen.OutputDir, gen.OutputOpt.Subdir),
			gen.OutputOpt.FilenameSuffix,
			bp.wb,
			bp.gen,
		)
		if err := be.export(); err != nil {
			return xerrors.WrapKV(err, xerrors.KeyBookName, debugBookName)
		}
	}
	return nil
}

// convertDocumentSheet processes a single document sheet within a book:
// it parses each top-level field node and appends the worksheet to the workbook.
// A sheet-level collector is used to accumulate field errors and enable fail-fast
// without aborting the entire book.
func (gen *Generator) convertDocumentSheet(bp *documentParser, bookCollector *xerrors.Collector, sheet *book.Sheet, debugBookName string) error {
	ws := sheet.ToWorkseet()
	debugSheetName := sheet.GetDebugName()
	log.Infof("%15s: %s", "parsing sheet", debugSheetName)

	// log.Debugf("dump document:\n%s", sheet.String())
	if len(sheet.Document.Children) != 1 {
		err := xerrors.Newf("document should have and only have one child (map node), sheet: %s", sheet.Name)
		return xerrors.WrapKV(err, xerrors.KeyBookName, debugBookName, xerrors.KeySheetName, debugSheetName)
	}

	sheetCollector := bookCollector.NewChild(gen.ErrorLimitOpt.MaxErrorsPerSheet,
		xerrors.KeySheetName, debugSheetName)
	// get the first child (map node) in document
	child := sheet.Document.Children[0]
	for _, node := range child.Children {
		if sheetCollector.IsFull() {
			return sheetCollector.Join()
		}
		field := &internalpb.Field{}
		parsed, err := bp.parseField(field, node)
		if err != nil {
			if err := sheetCollector.Collect(xerrors.WrapKV(err, xerrors.KeyBookName, debugBookName, xerrors.KeySheetName, debugSheetName)); err != nil {
				return err
			}
			continue
		}
		if parsed {
			ws.Fields = append(ws.Fields, field)
		}
	}
	if sheetCollector.HasErrors() {
		return sheetCollector.Join()
	}
	// append parsed sheet to workbook
	bp.wb.Worksheets = append(bp.wb.Worksheets, ws)
	return nil
}

// convertTableSheet processes a single sheet within a book during the two-pass flow:
//  1. first pass: extract type info from special sheet mode (non-default mode)
//  2. second pass: parse sheet schema and append worksheets to the workbook
func (gen *Generator) convertTableSheet(bp *tableParser, bookCollector *xerrors.Collector, sheet *book.Sheet, bookOpts *tableaupb.WorkbookOptions, debugBookName string, pass parsePass) error {
	ws := sheet.ToWorkseet()
	debugSheetName := sheet.GetDebugName()
	if pass == secondPass {
		log.Infof("%15s: %s", "parsing sheet", debugSheetName)
	}

	tableHeader := newTableHeader(ws.Options, bookOpts, gen.InputOpt.Header, sheet.Tabler())
	sheetCollector := bookCollector.NewChild(gen.ErrorLimitOpt.MaxErrorsPerSheet,
		xerrors.KeySheetName, debugSheetName)

	if pass == firstPass && ws.Options.Mode != tableaupb.Mode_MODE_DEFAULT {
		log.Debugf("first pass: extract type info from %s", debugSheetName)
		parentFilename := bp.GetProtoFilePath()
		err := gen.extractTypeInfoFromSpecialSheetMode(ws.Options.Mode, sheet, ws.Name, parentFilename)
		if err != nil {
			if err := sheetCollector.Collect(xerrors.WrapKV(err,
				xerrors.KeyBookName, debugBookName,
				xerrors.KeySheetName, debugSheetName)); err != nil {
				return err
			}
		}
	} else if pass == secondPass {
		log.Debugf("second pass: parse sheet schema from %s", debugSheetName)
		if ws.Options.Mode == tableaupb.Mode_MODE_DEFAULT {
			var parsed bool
			for cursor := 0; cursor < len(tableHeader.nameRowData); cursor++ {
				if sheetCollector.IsFull() {
					return bookCollector.Join()
				}
				field := &internalpb.Field{}
				var err error
				cursor, parsed, err = bp.parseField(field, tableHeader, cursor, "", "", tableparser.Nested(ws.Options.Nested))
				if err != nil {
					if err := sheetCollector.Collect(wrapDebugErr(err, debugBookName, debugSheetName, tableHeader, cursor)); err != nil {
						return err
					}
					return nil
				}
				if parsed {
					ws.Fields = append(ws.Fields, field)
				}
			}
			// append parsed sheet to workbook
			bp.wb.Worksheets = append(bp.wb.Worksheets, ws)
		} else {
			worksheets, err := gen.parseSpecialSheetMode(ws.Options.Mode, ws, sheet, debugBookName, debugSheetName, sheetCollector)
			if err != nil {
				return sheetCollector.Collect(xerrors.WrapKV(err,
					xerrors.KeyBookName, debugBookName,
					xerrors.KeySheetName, debugSheetName))
			}
			// append parsed sheets to workbook
			bp.wb.Worksheets = append(bp.wb.Worksheets, worksheets...)
		}
	}
	if sheetCollector.HasErrors() {
		return sheetCollector.Join()
	}
	return nil
}

func (gen *Generator) extractTypeInfoFromSpecialSheetMode(mode tableaupb.Mode, sheet *book.Sheet, typeName, parentFilename string) error {
	// create parser
	sheetOpts := &tableaupb.WorksheetOptions{
		Name:      sheet.Name,
		Namerow:   1,
		Datarow:   2,
		Transpose: sheet.Meta.GetTranspose(),
	}
	table := sheet.Tabler()
	parser := confgen.NewSheetParser(gen.ctx, xproto.InternalProtoPackage, gen.LocationName, sheetOpts)
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
		for row := table.BeginRow(); row < table.EndRow(); row++ {
			cols := table.GetRow(row)
			if isEnumTypeBlockHeader(cols) {
				if row < 1 {
					continue
				}
				typeRow := table.GetRow(row - 1)
				typeName, _, err := extractTableBlockTypeRow(typeRow)
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
		for row := table.BeginRow(); row < table.EndRow(); row++ {
			cols := table.GetRow(row)
			if isStructTypeBlockHeader(cols) {
				if row < 1 {
					continue
				}
				typeRow := table.GetRow(row - 1)
				typeName, _, err := extractTableBlockTypeRow(typeRow)
				if err != nil {
					return xerrors.Wrapf(err, "failed to parse struct type block at row: %d, sheet: %s", row, sheet.Name)
				}
				blockBeginRow := row
				blockEndRow := table.FindBlockEndRow(blockBeginRow)
				row = blockEndRow // skip row to next block
				subSheet := sheet.SubTableSheet(book.Rows(blockBeginRow, blockEndRow))
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
		for row := table.BeginRow(); row < table.EndRow(); row++ {
			cols := table.GetRow(row)
			if isUnionTypeBlockHeader(cols) {
				if row < 1 {
					continue
				}
				typeRow := table.GetRow(row - 1)
				typeName, _, err := extractTableBlockTypeRow(typeRow)
				if err != nil {
					return xerrors.Wrapf(err, "failed to parse union type block, sheet: %s, row: %d", sheet.Name, row)
				}
				blockBeginRow := row
				blockEndRow := table.FindBlockEndRow(blockBeginRow)
				row = blockEndRow // skip row to next block
				subSheet := sheet.SubTableSheet(book.Rows(blockBeginRow, blockEndRow))
				if err := extractUnionTypeInfo(subSheet, typeName, parentFilename, parser, gen); err != nil {
					return err
				}
			}
		}
	default:
		return xerrors.Newf("unknown mode: %v", mode)
	}
	return nil
}

func (gen *Generator) parseSpecialSheetMode(mode tableaupb.Mode, ws *internalpb.Worksheet, sheet *book.Sheet, debugBookName, debugSheetName string, sheetCollector *xerrors.Collector) ([]*internalpb.Worksheet, error) {
	// create parser
	sheetOpts := &tableaupb.WorksheetOptions{
		Name:      sheet.Name,
		Namerow:   1,
		Datarow:   2,
		Transpose: sheet.Meta.GetTranspose(),
	}
	table := sheet.Tabler()
	parser := confgen.NewSheetParser(gen.ctx, xproto.InternalProtoPackage, gen.LocationName, sheetOpts)

	// parse each special sheet mode
	switch mode {
	case tableaupb.Mode_MODE_ENUM_TYPE:
		if err := parseEnumType(ws, sheet, parser, gen); err != nil {
			return nil, err
		}
		return []*internalpb.Worksheet{ws}, nil
	case tableaupb.Mode_MODE_ENUM_TYPE_MULTI:
		var worksheets []*internalpb.Worksheet
		for row := table.BeginRow(); row < table.EndRow(); row++ {
			if sheetCollector.IsFull() {
				return nil, sheetCollector.Join()
			}
			cols := table.GetRow(row)
			if !isEnumTypeBlockHeader(cols) || row < 1 {
				continue
			}
			blockBeginRow := row
			blockErr := func() error {
				typeRow := table.GetRow(row - 1)
				subWs := proto.Clone(ws).(*internalpb.Worksheet)
				var err error
				subWs.Name, subWs.Note, err = extractTableBlockTypeRow(typeRow)
				if err != nil {
					return xerrors.Wrapf(err, "failed to extract enum type block at row: %d, sheet: %s", row, sheet.Name)
				}
				blockEndRow := table.FindBlockEndRow(blockBeginRow)
				row = blockEndRow // skip row to next block
				subSheet := sheet.SubTableSheet(book.Rows(blockBeginRow, blockEndRow))
				if err := parseEnumType(subWs, subSheet, parser, gen); err != nil {
					return err
				}
				worksheets = append(worksheets, subWs)
				return nil
			}()
			if blockErr != nil {
				if err := sheetCollector.Collect(xerrors.WrapKV(blockErr, xerrors.KeyBookName, debugBookName, xerrors.KeySheetName, debugSheetName)); err != nil {
					return nil, err
				}
			}
		}
		if sheetCollector.HasErrors() {
			return nil, sheetCollector.Join()
		}
		return worksheets, nil
	case tableaupb.Mode_MODE_STRUCT_TYPE:
		if err := parseStructType(ws, sheet, parser, gen, debugBookName, debugSheetName); err != nil {
			return nil, err
		}
		return []*internalpb.Worksheet{ws}, nil
	case tableaupb.Mode_MODE_STRUCT_TYPE_MULTI:
		var worksheets []*internalpb.Worksheet
		for row := table.BeginRow(); row < table.EndRow(); row++ {
			if sheetCollector.IsFull() {
				return nil, sheetCollector.Join()
			}
			cols := table.GetRow(row)
			if !isStructTypeBlockHeader(cols) || row < 1 {
				continue
			}
			blockBeginRow := row
			blockErr := func() error {
				typeRow := table.GetRow(row - 1)
				subWs := proto.Clone(ws).(*internalpb.Worksheet)
				var err error
				subWs.Name, subWs.Note, err = extractTableBlockTypeRow(typeRow)
				if err != nil {
					return xerrors.Wrapf(err, "failed to extract struct type block at row: %d, sheet: %s", row, sheet.Name)
				}
				blockEndRow := table.FindBlockEndRow(blockBeginRow)
				row = blockEndRow // skip row to next block
				subSheet := sheet.SubTableSheet(book.Rows(blockBeginRow, blockEndRow))
				if err := parseStructType(subWs, subSheet, parser, gen, debugBookName, debugSheetName); err != nil {
					return err
				}
				worksheets = append(worksheets, subWs)
				return nil
			}()
			if blockErr != nil {
				if err := sheetCollector.Collect(xerrors.WrapKV(blockErr, xerrors.KeyBookName, debugBookName, xerrors.KeySheetName, debugSheetName)); err != nil {
					return nil, err
				}
			}
		}
		if sheetCollector.HasErrors() {
			return nil, sheetCollector.Join()
		}
		return worksheets, nil
	case tableaupb.Mode_MODE_UNION_TYPE:
		if err := parseUnionType(ws, sheet, parser, gen, debugBookName, debugSheetName); err != nil {
			return nil, err
		}
		return []*internalpb.Worksheet{ws}, nil
	case tableaupb.Mode_MODE_UNION_TYPE_MULTI:
		// Each block clones the sheet options, including UnionShardSize.
		var worksheets []*internalpb.Worksheet
		for row := table.BeginRow(); row < table.EndRow(); row++ {
			if sheetCollector.IsFull() {
				return nil, sheetCollector.Join()
			}
			cols := table.GetRow(row)
			if !isUnionTypeBlockHeader(cols) || row < 1 {
				continue
			}
			blockBeginRow := row
			blockErr := func() error {
				typeRow := table.GetRow(row - 1)
				subWs := proto.Clone(ws).(*internalpb.Worksheet)
				var err error
				subWs.Name, subWs.Note, err = extractTableBlockTypeRow(typeRow)
				if err != nil {
					return xerrors.Wrapf(err, "failed to extract union type block at row: %d, sheet: %s", row, sheet.Name)
				}
				blockEndRow := table.FindBlockEndRow(blockBeginRow)
				row = blockEndRow // skip row to next block
				subSheet := sheet.SubTableSheet(book.Rows(blockBeginRow, blockEndRow))
				if err := parseUnionType(subWs, subSheet, parser, gen, debugBookName, debugSheetName); err != nil {
					return err
				}
				worksheets = append(worksheets, subWs)
				return nil
			}()
			if blockErr != nil {
				if err := sheetCollector.Collect(xerrors.WrapKV(blockErr, xerrors.KeyBookName, debugBookName, xerrors.KeySheetName, debugSheetName)); err != nil {
					return nil, err
				}
			}
		}
		if sheetCollector.HasErrors() {
			return nil, sheetCollector.Join()
		}
		return worksheets, nil
	default:
		return nil, xerrors.Newf("unknown mode: %v", mode)
	}
}
