package parseroptions

import (
	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/proto/tableaupb"
)

type Header struct {
	NameRow  int
	TypeRow  int
	NoteRow  int
	DataRow  int
	NameLine int
	TypeLine int
	NoteLine int
	Sep      string
	Subsep   string
}

// MergeHeader merges sheet-level, book-level, and global-level header options into the final header options.
func MergeHeader(sheetOpts *tableaupb.WorksheetOptions, bookOpts *tableaupb.WorkbookOptions, globalOpts *options.HeaderOption) *Header {
	hdr := &Header{}
	// name row
	if sheetOpts.GetNamerow() != 0 {
		hdr.NameRow = int(sheetOpts.GetNamerow())
	} else if bookOpts.GetNamerow() != 0 {
		hdr.NameRow = int(bookOpts.GetNamerow())
	} else if globalOpts != nil && globalOpts.NameRow != 0 {
		hdr.NameRow = int(globalOpts.NameRow)
	} else {
		hdr.NameRow = options.DefaultNameRow
	}
	// type row
	if sheetOpts.GetTyperow() != 0 {
		hdr.TypeRow = int(sheetOpts.GetTyperow())
	} else if bookOpts.GetTyperow() != 0 {
		hdr.TypeRow = int(bookOpts.GetTyperow())
	} else if globalOpts != nil && globalOpts.TypeRow != 0 {
		hdr.TypeRow = int(globalOpts.TypeRow)
	} else {
		hdr.TypeRow = options.DefaultTypeRow
	}
	// note row
	if sheetOpts.GetNoterow() != 0 {
		hdr.NoteRow = int(sheetOpts.GetNoterow())
	} else if bookOpts.GetNoterow() != 0 {
		hdr.NoteRow = int(bookOpts.GetNoterow())
	} else if globalOpts != nil && globalOpts.NoteRow != 0 {
		hdr.NoteRow = int(globalOpts.NoteRow)
	} else {
		hdr.NoteRow = options.DefaultNoteRow
	}
	// data row
	if sheetOpts.GetDatarow() != 0 {
		hdr.DataRow = int(sheetOpts.GetDatarow())
	} else if bookOpts.GetDatarow() != 0 {
		hdr.DataRow = int(bookOpts.GetDatarow())
	} else if globalOpts != nil && globalOpts.DataRow != 0 {
		hdr.DataRow = int(globalOpts.DataRow)
	} else {
		hdr.DataRow = options.DefaultDataRow
	}
	// name line
	if sheetOpts.GetNameline() != 0 {
		hdr.NameLine = int(sheetOpts.GetNameline())
	} else if bookOpts.GetNameline() != 0 {
		hdr.NameLine = int(bookOpts.GetNameline())
	} else if globalOpts != nil && globalOpts.NameLine != 0 {
		hdr.NameLine = int(globalOpts.NameLine)
	} else {
		hdr.NameLine = 0
	}
	// type line
	if sheetOpts.GetTypeline() != 0 {
		hdr.TypeLine = int(sheetOpts.GetTypeline())
	} else if bookOpts.GetTypeline() != 0 {
		hdr.TypeLine = int(bookOpts.GetTypeline())
	} else if globalOpts != nil && globalOpts.TypeLine != 0 {
		hdr.TypeLine = int(globalOpts.TypeLine)
	} else {
		hdr.TypeLine = 0
	}
	// note line
	if sheetOpts.GetNoteline() != 0 {
		hdr.NoteLine = int(sheetOpts.GetNoteline())
	} else if bookOpts.GetNoteline() != 0 {
		hdr.NoteLine = int(bookOpts.GetNoteline())
	} else if globalOpts != nil && globalOpts.NoteLine != 0 {
		hdr.NoteLine = int(globalOpts.NoteLine)
	} else {
		hdr.NoteLine = 0
	}
	// sep
	if sheetOpts.GetSep() != "" {
		hdr.Sep = sheetOpts.GetSep()
	} else if bookOpts.GetSep() != "" {
		hdr.Sep = bookOpts.GetSep()
	} else if globalOpts != nil && globalOpts.Sep != "" {
		hdr.Sep = globalOpts.Sep
	} else {
		hdr.Sep = options.DefaultSep
	}
	// subsep
	if sheetOpts.GetSubsep() != "" {
		hdr.Subsep = sheetOpts.GetSubsep()
	} else if bookOpts.GetSubsep() != "" {
		hdr.Subsep = bookOpts.GetSubsep()
	} else if globalOpts != nil && globalOpts.Subsep != "" {
		hdr.Subsep = globalOpts.Subsep
	} else {
		hdr.Subsep = options.DefaultSubsep
	}
	return hdr
}
