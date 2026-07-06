package protogen

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/proto/tableaupb/internalpb"
)

// TestDocumentParser_parseField_propagatesNote verifies that a note
// attached to a document node (typically extracted from a YAML `#`
// comment or an XML sibling comment) is propagated to the generated
// proto field's Note, so that the exporter can emit it as a `// ...`
// field comment.
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
