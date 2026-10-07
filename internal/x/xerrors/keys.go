package xerrors

const (
	ModuleDefault = "default"
	ModuleProto   = "protogen"
	ModuleConf    = "confgen"
)

// structured error keys
const (
	// Selects the localized summary template; values: default, proto, conf.
	KeyModule = "Module"

	KeyIndir            = "Indir"            // input dir
	KeySubdir           = "Subdir"           // input subdir
	KeyOutdir           = "Outdir"           // output dir
	KeyBookName         = "BookName"         // workbook name
	KeyPrimaryBookName  = "PrimaryBookName"  // primary workbook name
	KeySheetName        = "SheetName"        // worksheet name
	KeyPrimarySheetName = "PrimarySheetName" // primary worksheet name
	KeyReferBookName    = "ReferBookName"    // referred workbook name
	KeyReferSheetName   = "ReferSheetName"   // referred worksheet name

	KeyNameCellPos     = "NameCellPos"     // name cell position
	KeyNameCell        = "NameCell"        // name cell value
	KeyTrimmedNameCell = "TrimmedNameCell" // trimmed name cell value
	KeyTypeCellPos     = "TypeCellPos"     // type cell position
	KeyTypeCell        = "TypeCell"        // type cell value
	KeyNoteCellPos     = "NoteCellPos"     // note cell position
	KeyNoteCell        = "NoteCell"        // note cell value
	KeyDataCellPos     = "DataCellPos"     // data cell position
	KeyDataCell        = "DataCell"        // data data value

	KeyPBMessage   = "PBMessage"   // protobuf message name
	KeyPBFieldName = "PBFieldName" // protobuf message field name
	KeyPBFieldType = "PBFieldType" // protobuf message field type
	KeyPBFieldOpts = "PBFieldOpts" // protobuf message field options (extensions)
	KeyColumnName  = "ColumnName"  // column name

	KeyErrCode = "ErrCode"
	KeyErrDesc = "ErrDesc"
	KeyReason  = "Reason" // error reason
	// KeyHelp suggests how to fix the error.
	// See https://rustc-dev-guide.rust-lang.org/diagnostics.html#suggestions
	KeyHelp = "Help"
)

// debugKeyOrder defines the stable display order by referencing the key constants.
var debugKeyOrder = []string{
	KeyModule,

	KeyIndir,
	KeySubdir,
	KeyOutdir,
	KeyBookName,
	KeyPrimaryBookName,
	KeySheetName,
	KeyPrimarySheetName,
	KeyReferBookName,
	KeyReferSheetName,
	KeyNameCellPos,
	KeyNameCell,
	KeyTrimmedNameCell,
	KeyTypeCellPos,
	KeyTypeCell,
	KeyDataCellPos,
	KeyDataCell,

	KeyPBMessage,
	KeyPBFieldName,
	KeyPBFieldType,
	KeyPBFieldOpts,
	KeyColumnName,

	KeyErrCode,
	KeyErrDesc,
	KeyReason,
	KeyHelp,
}
