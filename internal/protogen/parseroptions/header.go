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
	Sep      string
	Subsep   string
}

// MergeHeader merges workbook options and worksheet options into final header options.
func MergeHeader(bookOpts *tableaupb.WorkbookOptions, sheetOpts *tableaupb.WorksheetOptions) *Header {
	hdr := &Header{}
	// name row
	if sheetOpts.GetNamerow() != 0 {
		hdr.NameRow = int(sheetOpts.GetNamerow())
	} else if bookOpts.GetNamerow() != 0 {
		hdr.NameRow = int(bookOpts.GetNamerow())
	} else {
		hdr.NameRow = options.DefaultNameRow
	}
	// type row
	if sheetOpts.GetTyperow() != 0 {
		hdr.TypeRow = int(sheetOpts.GetTyperow())
	} else if bookOpts.GetTyperow() != 0 {
		hdr.TypeRow = int(bookOpts.GetTyperow())
	} else {
		hdr.TypeRow = options.DefaultTypeRow
	}
	// note row
	if sheetOpts.GetNoterow() != 0 {
		hdr.NoteRow = int(sheetOpts.GetNoterow())
	} else if bookOpts.GetNoterow() != 0 {
		hdr.NoteRow = int(bookOpts.GetNoterow())
	} else {
		hdr.NoteRow = options.DefaultNoteRow
	}
	// data row
	if sheetOpts.GetDatarow() != 0 {
		hdr.DataRow = int(sheetOpts.GetDatarow())
	} else if bookOpts.GetDatarow() != 0 {
		hdr.DataRow = int(bookOpts.GetDatarow())
	} else {
		hdr.DataRow = options.DefaultDataRow
	}
	// name line
	if sheetOpts.GetNameline() != 0 {
		hdr.NameLine = int(sheetOpts.GetNameline())
	} else if bookOpts.GetNameline() != 0 {
		hdr.NameLine = int(bookOpts.GetNameline())
	} else {
		hdr.NameLine = 0
	}
	// type line
	if sheetOpts.GetTypeline() != 0 {
		hdr.TypeLine = int(sheetOpts.GetTypeline())
	} else if bookOpts.GetTypeline() != 0 {
		hdr.TypeLine = int(bookOpts.GetTypeline())
	} else {
		hdr.TypeLine = 0
	}
	return hdr
}
