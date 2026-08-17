package protogen

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/proto/tableaupb/internalpb"
)

// TestDocumentParser_parseField_propagatesNote verifies that a note
// attached to a document node (typically extracted from a YAML `#`
// line comment or an XML `note` attribute) is propagated to the
// generated proto field's Note, so that the exporter can emit it as a
// `// ...` field comment.
func TestDocumentParser_parseField_propagatesNote(t *testing.T) {
	dp := newDocumentParser("Test", "", "Test.yaml", testgen)

	t.Run("scalar field", func(t *testing.T) {
		node := &book.Node{
			Kind:  book.ScalarNode,
			Name:  "ID",
			Value: "uint32",
			Note:  "primary key",
		}
		field := &internalpb.Field{}
		parsed, err := dp.parseField(field, node)
		require.NoError(t, err)
		require.True(t, parsed)
		assert.Equal(t, "primary key", field.Note)
	})

	t.Run("scalar field with whitespace note is trimmed", func(t *testing.T) {
		node := &book.Node{
			Kind:  book.ScalarNode,
			Name:  "Name",
			Value: "string",
			Note:  "  display name  ",
		}
		field := &internalpb.Field{}
		parsed, err := dp.parseField(field, node)
		require.NoError(t, err)
		require.True(t, parsed)
		assert.Equal(t, "display name", field.Note)
	})

	t.Run("scalar field without note", func(t *testing.T) {
		node := &book.Node{
			Kind:  book.ScalarNode,
			Name:  "Score",
			Value: "int32",
		}
		field := &internalpb.Field{}
		parsed, err := dp.parseField(field, node)
		require.NoError(t, err)
		require.True(t, parsed)
		assert.Empty(t, field.Note)
	})

	t.Run("list field", func(t *testing.T) {
		// Equivalent to YAML:
		//   Items:
		//     "@type": "[Item]"
		//     "@struct":
		//       ID: uint32
		node := &book.Node{
			Kind: book.MapNode,
			Name: "Items",
			Note: "player inventory",
			Children: []*book.Node{
				{Kind: book.ScalarNode, Name: "@type", Value: "[Item]"},
				{
					Kind: book.MapNode,
					Name: "@struct",
					Children: []*book.Node{
						{Kind: book.ScalarNode, Name: "ID", Value: "uint32"},
					},
				},
			},
		}
		field := &internalpb.Field{}
		parsed, err := dp.parseField(field, node)
		require.NoError(t, err)
		require.True(t, parsed)
		assert.Equal(t, "player inventory", field.Note)
	})

	t.Run("sub-fields fabricated internally have no note", func(t *testing.T) {
		// The struct member ID has no note; verify it stays empty even
		// though the parent list carries a note.
		node := &book.Node{
			Kind: book.MapNode,
			Name: "Items",
			Note: "player inventory",
			Children: []*book.Node{
				{Kind: book.ScalarNode, Name: "@type", Value: "[Item]"},
				{
					Kind: book.MapNode,
					Name: "@struct",
					Children: []*book.Node{
						{Kind: book.ScalarNode, Name: "ID", Value: "uint32"},
					},
				},
			},
		}
		field := &internalpb.Field{}
		_, err := dp.parseField(field, node)
		require.NoError(t, err)
		require.NotEmpty(t, field.Fields)
		for _, sub := range field.Fields {
			assert.Empty(t, sub.Note, "sub-field %q should have no note", sub.Name)
		}
	})
}

func parseDocumentMapField(t *testing.T, node *book.Node) (*internalpb.Field, error) {
	t.Helper()
	registerVpropTypes(testgen)
	dp := newDocumentParser("Test", "", "Test.yaml", testgen)
	field := &internalpb.Field{}
	parsed, err := dp.parseField(field, node)
	if err != nil {
		return field, err
	}
	require.True(t, parsed)
	return field, nil
}

func incellMapNode(name, typ string) *book.Node {
	return &book.Node{
		Kind: book.MapNode,
		Name: name,
		Children: []*book.Node{
			{Kind: book.ScalarNode, Name: book.KeywordType, Value: typ},
			{Kind: book.ScalarNode, Name: book.KeywordIncell, Value: "true"},
		},
	}
}

func TestDocumentParser_incellScalarMapSetsVprop(t *testing.T) {
	field, err := parseDocumentMapField(t, incellMapNode("RangeMap", `map<int32, int32>||{range:"1,10"}`))
	require.NoError(t, err)
	require.NotNil(t, field.Options)
	assert.Equal(t, tableaupb.Layout_LAYOUT_INCELL, field.Options.Layout)
	require.NotNil(t, field.Options.Vprop)
	assert.Equal(t, "1,10", field.Options.Vprop.Range)
}

func TestDocumentParser_incellScalarMapSetsKeyAndValueProp(t *testing.T) {
	field, err := parseDocumentMapField(t, incellMapNode("ComboMap", `map<int32, int64>|{refer:"ItemConf.ID"}|{range:"1,10"}`))
	require.NoError(t, err)
	// Document maps extract map-level prop via ExtractMapFieldProp, which
	// does not keep refer on the map field (refer stays on the key type).
	require.NotNil(t, field.Options.Vprop)
	assert.Equal(t, "1,10", field.Options.Vprop.Range)
}

func TestDocumentParser_enumKeyIncellMapSinksVpropToValueField(t *testing.T) {
	field, err := parseDocumentMapField(t, incellMapNode("EnumRangeMap", `map<enum<.FruitType>, int32>||{range:"1,10"}`))
	require.NoError(t, err)
	require.NotNil(t, field.Options)
	assert.Nil(t, field.Options.Vprop, "vprop must be sunk into the generated Value field")
	require.Len(t, field.Fields, 2)
	valueField := field.Fields[1]
	assert.Equal(t, book.KeywordValue, valueField.Options.GetName())
	require.NotNil(t, valueField.Options.GetProp())
	assert.Equal(t, "1,10", valueField.Options.Prop.Range)
}

func TestDocumentParser_invalidVpropReturnsError(t *testing.T) {
	_, err := parseDocumentMapField(t, incellMapNode("BadVprop", `map<int32, int32>||{range:not-a-prop}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse field prop")
}

func TestDocumentParser_vpropOnNonIncellScalarMapIsRejected(t *testing.T) {
	node := &book.Node{
		Kind:  book.ScalarNode,
		Name:  "RangeMap",
		Value: `map<int32, int32>||{range:"1,10"}`,
	}
	_, err := parseDocumentMapField(t, node)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "vprop is only valid for incell scalar maps")
}

func TestDocumentParser_nonIncellScalarMapHasNoVprop(t *testing.T) {
	node := &book.Node{
		Kind:  book.ScalarNode,
		Name:  "ScalarMap",
		Value: `map<int32, int32>`,
	}
	field, err := parseDocumentMapField(t, node)
	require.NoError(t, err)
	require.NotNil(t, field.Options)
	assert.Nil(t, field.Options.Vprop)
	assert.NotEqual(t, tableaupb.Layout_LAYOUT_INCELL, field.Options.Layout)
}
