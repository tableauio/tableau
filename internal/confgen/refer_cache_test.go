package confgen

import (
	"context"
	"testing"

	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/proto/tableaupb"
)

func TestGeneratorIsSingleUse(t *testing.T) {
	gen := NewGeneratorWithOptions("", "", t.TempDir(), options.NewDefault())
	if err := gen.run(func() error { return nil }); err != nil {
		t.Fatalf("first run error = %v", err)
	}
	if err := gen.run(func() error {
		t.Fatal("second run callback was called")
		return nil
	}); err == nil {
		t.Fatal("second run error = nil, want single-use error")
	}
}

func TestNewExtendedSheetParserInitializesReferredCache(t *testing.T) {
	extInfo := &SheetParserExtInfo{}
	NewExtendedSheetParser(
		context.Background(),
		"unittest",
		"Asia/Shanghai",
		&tableaupb.WorkbookOptions{},
		&tableaupb.WorksheetOptions{},
		extInfo,
	)
	if extInfo.ReferredCache == nil {
		t.Fatal("NewExtendedSheetParser() left ReferredCache nil")
	}

	cache := extInfo.ReferredCache
	NewExtendedSheetParser(
		context.Background(),
		"unittest",
		"Asia/Shanghai",
		&tableaupb.WorkbookOptions{},
		&tableaupb.WorksheetOptions{},
		extInfo,
	)
	if extInfo.ReferredCache != cache {
		t.Fatal("NewExtendedSheetParser() replaced an existing ReferredCache")
	}
}
