package i18n

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
