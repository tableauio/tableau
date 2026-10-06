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
	require.ErrorIs(t, wrapped, xerrors.ErrE2031)
	assert.Same(t, structured, tableau.WrapError(structured))
	assert.Equal(t, xerrors.NewDesc(err).String(), structured.Error())
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
	assert.Equal(t, structured.Details, decoded.Details)
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
	assert.Equal(t, structured.Details, decoded.Details)
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
