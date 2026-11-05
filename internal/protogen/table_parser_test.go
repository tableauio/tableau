package protogen

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/importer/book/tableparser"
	"github.com/tableauio/tableau/internal/types"
	"github.com/tableauio/tableau/internal/x/xproto"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/proto/tableaupb/internalpb"
)

var registerVpropTypesOnce sync.Once

func registerVpropTypes(gen *Generator) {
	registerVpropTypesOnce.Do(func() {
		gen.typeInfos.Put(&xproto.TypeInfo{
			FullName:       "protoconf.FruitType",
			ParentFilename: "common.proto",
			Kind:           types.EnumKind,
		})
		gen.typeInfos.Put(&xproto.TypeInfo{
			FullName:       "protoconf.Item",
			ParentFilename: "common.proto",
			Kind:           types.MessageKind,
		})
	})
}

func newVpropTableHeader(name, typ, note string) *tableHeader {
	return &tableHeader{
		Header: &tableparser.Header{
			NameRow: 1,
			TypeRow: 2,
			NoteRow: 3,
		},
		Positioner:  &book.Table{},
		nameRowData: []string{name},
		typeRowData: []string{typ},
		noteRowData: []string{note},
		validNames:  map[string]int{},
	}
}

func parseTableMapField(t *testing.T, name, typ string) (*internalpb.Field, error) {
	t.Helper()
	registerVpropTypes(testgen)
	tp := newTableParser("Test", "", "Test.xlsx", testgen)
	field := &internalpb.Field{}
	_, parsed, err := tp.parseField(field, newVpropTableHeader(name, typ, "note"), 0, "", "")
	if err != nil {
		return field, err
	}
	require.True(t, parsed)
	return field, nil
}

func TestTableParser_incellScalarMapSetsVprop(t *testing.T) {
	field, err := parseTableMapField(t, "RangeMap", `map<int32, int32>||{range:"1,10"}`)
	require.NoError(t, err)
	require.NotNil(t, field.Options)
	assert.Equal(t, tableaupb.Layout_LAYOUT_INCELL, field.Options.Layout)
	require.NotNil(t, field.Options.Vprop)
	assert.Equal(t, "1,10", field.Options.Vprop.Range)
	assert.Nil(t, field.Options.Prop)
}

func TestTableParser_incellScalarMapSetsKeyAndValueProp(t *testing.T) {
	field, err := parseTableMapField(t, "ComboMap", `map<int32, int64>|{refer:"ItemConf.ID"}|{range:"1,10"}`)
	require.NoError(t, err)
	require.NotNil(t, field.Options)
	require.NotNil(t, field.Options.Prop)
	assert.Equal(t, "ItemConf.ID", field.Options.Prop.Refer)
	require.NotNil(t, field.Options.Vprop)
	assert.Equal(t, "1,10", field.Options.Vprop.Range)
}

func TestTableParser_enumKeyIncellMapSinksVpropToValueField(t *testing.T) {
	field, err := parseTableMapField(t, "EnumRangeMap", `map<enum<.FruitType>, int32>||{range:"1,10"}`)
	require.NoError(t, err)
	require.NotNil(t, field.Options)
	assert.Nil(t, field.Options.Vprop, "vprop must be sunk into the generated Value field")
	require.Len(t, field.Fields, 2)
	valueField := field.Fields[1]
	assert.Equal(t, types.DefaultMapValueOptName, valueField.Options.GetName())
	require.NotNil(t, valueField.Options.GetProp())
	assert.Equal(t, "1,10", valueField.Options.Prop.Range)
}

func TestTableParser_invalidVpropReturnsError(t *testing.T) {
	_, err := parseTableMapField(t, "BadVprop", `map<int32, int32>||{range:not-a-prop}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse field prop")
}

func TestTableParser_vpropOnVerticalStructMapIsRejected(t *testing.T) {
	_, err := parseTableMapField(t, "ID", `map<int32, .Item>||{range:"1,10"}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "vprop is only valid for incell scalar maps")
}
