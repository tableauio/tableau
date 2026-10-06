package diagnostic_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau/diagnostic"
	"github.com/tableauio/tableau/internal/x/xerrors"
)

func TestNewDesc(t *testing.T) {
	assert.Nil(t, diagnostic.NewDesc(nil))
	plain := fmt.Errorf("load config: %w", errors.New("unavailable"))
	d := diagnostic.NewDesc(plain)
	require.NotNil(t, d)
	assert.Equal(t, plain.Error(), d.String())
	assert.Empty(t, d.Fields())
	assert.Empty(t, d.Children())
}

func TestIncellStructDiagnostic(t *testing.T) {
	cause := xerrors.E2031("protoconf.FightTypeFilter", ";ranked;casual", 2, 3, ";")
	err := xerrors.WrapKV(cause,
		xerrors.KeyModule, xerrors.ModuleConf,
		xerrors.KeyBookName, "server/BattlePass.xlsx",
		xerrors.KeyPrimaryBookName, "server/Task.xlsx",
		xerrors.KeySheetName, "TaskConfig",
		xerrors.KeyDataCellPos, "L6",
		xerrors.KeyDataCell, ";ranked;casual")
	d := diagnostic.NewDesc(fmt.Errorf("load failed: %w", err))
	assert.Equal(t, xerrors.NewDesc(err).String(), d.String())
	assert.Contains(t, d.String(), "Workbook: server/BattlePass.xlsx (Primary: server/Task.xlsx)")
	assert.Contains(t, d.String(), "DataCellPos: L6")
	assert.NotContains(t, d.String(), "--- debugging ---")
	assert.Equal(t, "E2031", d.GetValue("ErrCode"))

	fields := d.Fields()
	fields["BookName"] = "changed.xlsx"
	assert.Equal(t, "server/BattlePass.xlsx", d.GetValue("BookName"), "Fields must return a copy")
	b, marshalErr := json.Marshal(d)
	require.NoError(t, marshalErr)
	var decoded struct {
		Fields map[string]any `json:"fields"`
	}
	require.NoError(t, json.Unmarshal(b, &decoded))
	assert.Equal(t, "server/BattlePass.xlsx", decoded.Fields["BookName"])
	assert.Equal(t, "server/Task.xlsx", decoded.Fields["PrimaryBookName"])
	assert.Equal(t, "L6", decoded.Fields["DataCellPos"])
	assert.Equal(t, "E2031", decoded.Fields["ErrCode"])
	assert.NotContains(t, string(b), "stack")
}

func TestJoinedDiagnostics(t *testing.T) {
	root := xerrors.NewCollector(10)
	for _, book := range []string{"Shard1.xlsx", "Shard2.xlsx"} {
		sheet := root.NewChild(0,
			xerrors.KeyModule, xerrors.ModuleConf,
			xerrors.KeyBookName, book,
			xerrors.KeySheetName, "Tasks",
			xerrors.KeyPrimaryBookName, "Main.xlsx")
		_ = sheet.Collect(xerrors.WrapKV(xerrors.E2005(book), xerrors.KeyDataCellPos, "A4"))
	}
	d := diagnostic.NewDesc(fmt.Errorf("load: %w", root.Join()))
	children := d.Children()
	require.Len(t, children, 2)
	for i, child := range children {
		assert.Equal(t, fmt.Sprintf("Shard%d.xlsx", i+1), child.GetValue("BookName"))
		assert.Equal(t, "Main.xlsx", child.GetValue("PrimaryBookName"))
		assert.Equal(t, "A4", child.GetValue("DataCellPos"))
	}
	children[0] = nil
	require.NotNil(t, d.Children()[0], "Children must return a copy")
	b, err := json.Marshal(d)
	require.NoError(t, err)
	var decoded struct {
		Children []struct {
			Fields map[string]any `json:"fields"`
		} `json:"children"`
	}
	require.NoError(t, json.Unmarshal(b, &decoded))
	require.Len(t, decoded.Children, 2)
	assert.Equal(t, "Shard1.xlsx", decoded.Children[0].Fields["BookName"])
	assert.Equal(t, "Shard2.xlsx", decoded.Children[1].Fields["BookName"])
	assert.Empty(t, d.Fields(), "aggregate must not synthesize fields from different leaves")
}

func TestJoinedPlainErrorsJSON(t *testing.T) {
	d := diagnostic.NewDesc(errors.Join(errors.New("first"), errors.New("second")))
	b, err := json.Marshal(d)
	require.NoError(t, err)
	assert.JSONEq(t, `{"children":[{"message":"first"},{"message":"second"}]}`, string(b))
}
