package tableau_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau"
	"github.com/tableauio/tableau/internal/x/xerrors"
)

func TestWrapError(t *testing.T) {
	assert.Nil(t, tableau.WrapError(nil))
	plain := fmt.Errorf("load config: %w", errors.New("unavailable"))
	assert.Same(t, plain, tableau.WrapError(plain))

	cause := xerrors.E2031("protoconf.FightTypeFilter", ";ranked;casual", 2, 3, ";")
	err := xerrors.WrapKV(cause,
		xerrors.KeyModule, xerrors.ModuleConf,
		xerrors.KeyBookName, "server/BattlePass.xlsx",
		xerrors.KeyPrimaryBookName, "server/Task.xlsx",
		xerrors.KeySheetName, "TaskConfig",
		xerrors.KeyDataCellPos, "L6",
		xerrors.KeyDataCell, ";ranked;casual",
		xerrors.KeyPBMessage, "TaskConfig",
		xerrors.KeyPBFieldName, "fight_type_filter")
	wrapped := tableau.WrapError(fmt.Errorf("load failed: %w", err))
	var structured *tableau.Error
	require.ErrorAs(t, wrapped, &structured)
	var internal *xerrors.Error
	require.ErrorAs(t, wrapped, &internal)
	assert.Same(t, structured, internal, "public and internal errors must be the same object")
	require.ErrorIs(t, wrapped, xerrors.ErrE2031)
	assert.Same(t, structured, tableau.WrapError(structured))
	assert.Equal(t, xerrors.NewError(err).Error(), structured.Error())
	assert.NotContains(t, structured.Error(), "--- debugging ---")
	assert.Contains(t, fmt.Sprintf("%+v", structured), "--- debugging ---")
	require.Len(t, structured.Details, 1)
	detail := structured.Details[0]
	assert.Equal(t, "E2031", detail.Code)
	assert.Contains(t, detail.Message, "protoconf.FightTypeFilter")
	require.NotNil(t, detail.Source)
	assert.Equal(t, "server/BattlePass.xlsx", detail.Source.Workbook)
	assert.Equal(t, "server/Task.xlsx", detail.Source.PrimaryWorkbook)
	assert.Equal(t, "TaskConfig", detail.Source.Worksheet)
	assert.Equal(t, &tableau.CellLocation{Position: "L6", Data: ";ranked;casual"}, detail.Source.Cell)
	assert.Equal(t, "TaskConfig", detail.Field.Message)
	assert.Equal(t, "fight_type_filter", detail.Field.Name)

	b, marshalErr := json.Marshal(structured)
	require.NoError(t, marshalErr)
	var decoded tableau.Error
	require.NoError(t, json.Unmarshal(b, &decoded))
	roundtrip, marshalErr := json.Marshal(&decoded)
	require.NoError(t, marshalErr)
	assert.JSONEq(t, string(b), string(roundtrip))
	assert.Equal(t, structured.Error(), decoded.Error())
	assert.NotContains(t, string(b), "stack")
	assert.NotContains(t, string(b), `"fields"`)
	assert.NotContains(t, string(b), `"children"`)
}

func TestErrorDetailsFromCollector(t *testing.T) {
	root := xerrors.NewCollector(10)
	for _, book := range []string{"Shard1.xlsx", "Shard2.xlsx"} {
		sheet := root.NewChild(0,
			xerrors.KeyModule, xerrors.ModuleConf,
			xerrors.KeyBookName, book,
			xerrors.KeySheetName, "Tasks",
			xerrors.KeyPrimaryBookName, "Main.xlsx")
		_ = sheet.Collect(xerrors.WrapKV(xerrors.E2005(book), xerrors.KeyDataCellPos, "A4"))
	}
	plain := errors.New("connection closed")
	wrapped := tableau.WrapError(errors.Join(fmt.Errorf("load: %w", root.Join()), plain))
	var structured *tableau.Error
	require.ErrorAs(t, wrapped, &structured)
	require.Len(t, structured.Details, 3)
	for i, detail := range structured.Details[:2] {
		assert.Equal(t, "E2005", detail.Code)
		assert.Equal(t, fmt.Sprintf("Shard%d.xlsx", i+1), detail.Source.Workbook)
		assert.Equal(t, "Main.xlsx", detail.Source.PrimaryWorkbook)
		assert.Equal(t, "A4", detail.Source.Cell.Position)
	}
	assert.Equal(t, "connection closed", structured.Details[2].Message)
	assert.Nil(t, structured.Details[2].Source)
	require.ErrorIs(t, wrapped, plain)
	b, err := json.Marshal(structured)
	require.NoError(t, err)
	var decoded tableau.Error
	require.NoError(t, json.Unmarshal(b, &decoded))
	roundtrip, marshalErr := json.Marshal(&decoded)
	require.NoError(t, marshalErr)
	assert.JSONEq(t, string(b), string(roundtrip))
	assert.Equal(t, structured.Error(), decoded.Error())
}

func TestErrorDetailsNormalization(t *testing.T) {
	err := xerrors.NewKV("invalid header",
		xerrors.KeyModule, xerrors.ModuleProto,
		xerrors.KeyBookName, "Items.xlsx",
		xerrors.KeyNameCellPos, "A1",
		xerrors.KeyNameCell, " ID ",
		xerrors.KeyTrimmedNameCell, "ID",
		xerrors.KeyTypeCellPos, "A2",
		xerrors.KeyTypeCell, "invalid-type",
		xerrors.KeyNoteCellPos, "A3",
		xerrors.KeyNoteCell, "identifier")
	var structured *tableau.Error
	require.ErrorAs(t, tableau.WrapError(err), &structured)
	require.Len(t, structured.Details, 1)
	detail := structured.Details[0]
	assert.Equal(t, "E0004", detail.Code)
	assert.Equal(t, &tableau.CellLocation{Position: "A1", Data: " ID ", TrimmedData: "ID"}, detail.Source.NameCell)
	assert.Equal(t, "A2", detail.Source.TypeCell.Position)
	assert.Equal(t, "identifier", detail.Source.NoteCell.Data)
	before, err := json.Marshal(structured)
	require.NoError(t, err)
	for range 3 {
		_ = structured.Error()
	}
	after, err := json.Marshal(structured)
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after), "rendering must not change the serialized data")
}

// A consumer editing typed details must see the same values in text and JSON;
// there must be no stale descriptor or cached summary behind the public error.
func TestErrorDetailsDriveRendering(t *testing.T) {
	cause := xerrors.WrapKV(xerrors.E2005("duplicate"),
		xerrors.KeyModule, xerrors.ModuleConf,
		xerrors.KeyBookName, "Old.xlsx", xerrors.KeyDataCellPos, "A4")
	var e *tableau.Error
	require.ErrorAs(t, tableau.WrapError(cause), &e)
	e.Details[0].Source.Workbook = "New.xlsx"
	e.Details[0].Source.Cell.Position = "B6"
	e.Details[0].Message = "updated reason"
	assert.Contains(t, e.Error(), "Workbook: New.xlsx")
	assert.Contains(t, e.Error(), "DataCellPos: B6")
	assert.Contains(t, e.Error(), "Reason: updated reason")
	assert.NotContains(t, e.Error(), "Old.xlsx")
	assert.Equal(t, e.Details[0].String(), e.Error())

	// Joining an existing typed error must use those same edited details and
	// retain its original cause through standard Go error traversal.
	joined := tableau.WrapError(errors.Join(fmt.Errorf("load: %w", e), errors.New("offline")))
	var aggregate *tableau.Error
	require.ErrorAs(t, joined, &aggregate)
	require.Len(t, aggregate.Details, 2)
	assert.Contains(t, aggregate.Error(), "Workbook: New.xlsx")
	require.ErrorIs(t, joined, xerrors.ErrE2005)
	encoded, err := json.Marshal(aggregate)
	require.NoError(t, err)
	var decoded tableau.Error
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	assert.Equal(t, aggregate.Error(), decoded.Error())
}

func TestErrorPlainAndEmptyDetails(t *testing.T) {
	plain := xerrors.Wrapf(xerrors.New("inner"), "outer")
	assert.Equal(t, "outer: inner", tableau.WrapError(plain).Error())
	scoped := xerrors.WrapKV(errors.New("offline"), xerrors.KeyModule, xerrors.ModuleConf, xerrors.KeyBookName, "Main.xlsx")
	assert.Equal(t, "offline", tableau.WrapError(scoped).Error())
	for _, e := range []*tableau.Error{nil, {}, {Details: []*tableau.ErrorDetail{nil}}, {Details: []*tableau.ErrorDetail{{}}}} {
		assert.Empty(t, e.Error())
		assert.NotPanics(t, func() { _ = tableau.WrapError(fmt.Errorf("wrapped: %w", e)) })
	}
}
