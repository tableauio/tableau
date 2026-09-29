package confgen

import (
	"context"
	"testing"

	"github.com/tableauio/tableau/proto/tableaupb"
)

func TestNewExtendedSheetParserInitializesReferredCache(t *testing.T) {
	extInfo := &SheetParserExtInfo{}
	NewExtendedSheetParser(
		context.Background(),
		"unittest",
		"Asia/Shanghai",
		&tableaupb.WorkbookOptions{},
		&tableaupb.WorksheetOptions{},
		extInfo,
		SourceLocation{},
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
		SourceLocation{},
	)
	if extInfo.ReferredCache != cache {
		t.Fatal("NewExtendedSheetParser() replaced an existing ReferredCache")
	}
}
