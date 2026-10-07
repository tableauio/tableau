package tableau

import "github.com/tableauio/tableau/internal/x/xerrors"

// Module names select the localized error summary format.
const (
	ModuleDefault = xerrors.ModuleDefault
	ModuleProto   = xerrors.ModuleProto
	ModuleConf    = xerrors.ModuleConf
)

// Metadata keys for NewKV, WrapKV, and GetValue.
// Inspect maps these fields to the typed ErrorDetail representation.
const (
	KeyModule           = xerrors.KeyModule
	KeyIndir            = xerrors.KeyIndir            // input dir
	KeySubdir           = xerrors.KeySubdir           // input subdir
	KeyOutdir           = xerrors.KeyOutdir           // output dir
	KeyBookName         = xerrors.KeyBookName         // workbook name
	KeyPrimaryBookName  = xerrors.KeyPrimaryBookName  // primary workbook name
	KeySheetName        = xerrors.KeySheetName        // worksheet name
	KeyPrimarySheetName = xerrors.KeyPrimarySheetName // primary worksheet name
	KeyReferBookName    = xerrors.KeyReferBookName    // referred workbook name
	KeyReferSheetName   = xerrors.KeyReferSheetName   // referred worksheet name
	KeyNameCellPos      = xerrors.KeyNameCellPos      // name cell position
	KeyNameCell         = xerrors.KeyNameCell         // name cell value
	KeyTrimmedNameCell  = xerrors.KeyTrimmedNameCell  // trimmed name cell value
	KeyTypeCellPos      = xerrors.KeyTypeCellPos      // type cell position
	KeyTypeCell         = xerrors.KeyTypeCell         // type cell value
	KeyNoteCellPos      = xerrors.KeyNoteCellPos      // note cell position
	KeyNoteCell         = xerrors.KeyNoteCell         // note cell value
	KeyDataCellPos      = xerrors.KeyDataCellPos      // data cell position
	KeyDataCell         = xerrors.KeyDataCell         // data cell value
	KeyPBMessage        = xerrors.KeyPBMessage        // protobuf message name
	KeyPBFieldName      = xerrors.KeyPBFieldName      // protobuf message field name
	KeyPBFieldType      = xerrors.KeyPBFieldType      // protobuf message field type
	KeyPBFieldOpts      = xerrors.KeyPBFieldOpts      // protobuf message field options (extensions)
	KeyColumnName       = xerrors.KeyColumnName       // column name
	KeyErrCode          = xerrors.KeyErrCode
	KeyErrDesc          = xerrors.KeyErrDesc
	KeyReason           = xerrors.KeyReason // error reason
	KeyHelp             = xerrors.KeyHelp
)
