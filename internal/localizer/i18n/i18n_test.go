package i18n

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

func Test_RenderEcode(t *testing.T) {
	bundles, err := loadBundles(supportedLangs)
	assert.NoError(t, err)
	i18n := I18N{bundles: bundles}
	fields := map[string]any{
		"SheetName": "Sheet1",
		"BookName":  "Book1",
	}
	result := &EcodeDetail{
		Desc: "sheet not found in book",
		Text: `sheet "Sheet1" not found in book "Book1"`,
	}
	e0001 := i18n.RenderEcode(language.English, "E0001", fields)
	assert.Equal(t, result, e0001)

	// not supported lang
	notSupportedLang := language.Indonesian
	bundles[notSupportedLang.String()] = &Bundle{
		lang: notSupportedLang,
	}
	notFoundE0001 := i18n.RenderEcode(notSupportedLang, "E0001", fields)
	assert.Equal(t, result, notFoundE0001)
}

func Test_RenderMessage(t *testing.T) {
	bundles, err := loadBundles(supportedLangs)
	assert.NoError(t, err)
	i18n := I18N{bundles: bundles}
	fields := map[string]any{
		"ErrCode": "E0001",
		"ErrDesc": "sheet not found in book",
		"Reason":  `sheet "Sheet1" not found in book "Book1"`,
	}
	result := `error[E0001]: sheet not found in book
Reason: sheet "Sheet1" not found in book "Book1"
`
	e0001 := i18n.RenderMessage(language.English, "default", fields)
	assert.Equal(t, result, e0001)

	// not supported lang
	notSupportedLang := language.Indonesian
	bundles[notSupportedLang.String()] = &Bundle{
		lang: notSupportedLang,
	}
	notFound := i18n.RenderMessage(notSupportedLang, "default", fields)
	assert.Equal(t, result, notFound)
}

func Test_EcodeField(t *testing.T) {
	field := EcodeField{"SheetName": "string"}
	assert.True(t, field.Validate())
	assert.Equal(t, "SheetName", field.Name())
	assert.Equal(t, "string", field.Type())

	field2 := EcodeField{"Time": "time.Time"}
	assert.True(t, field2.Validate())
	assert.Equal(t, "Time", field2.Name())
	assert.Equal(t, "time.Time", field2.Type())
	assert.Equal(t, "time", field2.ImportPath())

	field3 := EcodeField{"DateTime": "google.golang.org/protobuf/types/known/timestamppb.Timestamp"}
	assert.True(t, field3.Validate())
	assert.Equal(t, "DateTime", field3.Name())
	assert.Equal(t, "timestamppb.Timestamp", field3.Type())
	assert.Equal(t, "google.golang.org/protobuf/types/known/timestamppb", field3.ImportPath())

	invalidField := EcodeField{}
	invalidField1 := EcodeField{"SheetName": ""}
	invalidField2 := EcodeField{"": "string"}
	invalidField3 := EcodeField{"SheetName": "string", "BookName": "string"}
	assert.False(t, invalidField.Validate())
	assert.False(t, invalidField1.Validate())
	assert.False(t, invalidField2.Validate())
	assert.False(t, invalidField3.Validate())
}

func Test_loadBundles(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		langs   []language.Tag
		wantErr bool
	}{
		{
			name: "test",
			langs: []language.Tag{
				language.English,
				language.Chinese,
			},
			wantErr: false,
		},
		{
			name: "test",
			langs: []language.Tag{
				language.English,
				language.Chinese,
				language.Spanish,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, gotErr := loadBundles(tt.langs)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("loadBundles() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("loadBundles() succeeded unexpectedly")
			}
		})
	}
}

func TestFormatSource(t *testing.T) {
	for _, tt := range []struct {
		name            string
		source, primary any
		attributes      []any
		want            string
	}{
		{name: "bare source", source: "Items.xlsx", want: "Items.xlsx"},
		{name: "matching primary", source: "Items.xlsx", primary: "Items.xlsx", want: "Items.xlsx"},
		{name: "absent attributes", source: "Items.xlsx", attributes: []any{"Alias", nil, "Merger", []string{}}, want: "Items.xlsx"},
		{name: "metadata without physical name", attributes: []any{"Alias", "Items"}, want: "(Alias: Items)"},
		{name: "combined attributes", source: "Shard.xlsx", primary: "Items.xlsx",
			attributes: []any{"Alias", "Items", "Merger", []string{"Shard*.xlsx", "Extra.xlsx"}, "Scatter", []string{"Patch*.xlsx"}},
			want:       "Shard.xlsx (Primary: Items.xlsx, Alias: Items, Merger: [Shard*.xlsx, Extra.xlsx], Scatter: [Patch*.xlsx])"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := formatSource(tt.source, tt.primary, "Primary", tt.attributes...)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
	_, err := formatSource("Items.xlsx", nil, "Primary", "Alias")
	require.Error(t, err)
}
