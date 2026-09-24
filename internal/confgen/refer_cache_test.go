package confgen

import (
	"context"
	"errors"
	"testing"

	"github.com/tableauio/tableau/internal/confgen/fieldprop"
	"github.com/tableauio/tableau/internal/x/xerrors"
	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/proto/tableaupb"
	_ "github.com/tableauio/tableau/proto/tableaupb/unittestpb"
	"google.golang.org/protobuf/reflect/protoregistry"
)

func TestGeneratorResetRunStateRetriesFailedRefer(t *testing.T) {
	gen := &Generator{
		ErrorLimitOpt: &options.ErrorLimitOption{MaxErrors: 10},
		collector:     xerrors.NewCollector(10),
		referredCache: fieldprop.NewReferredCache(),
	}
	prop := &tableaupb.FieldProp{Refer: "DoesNotExistConf.ID"}
	input := &fieldprop.Input{
		ProtoPackage: "unittest",
		InputDir:     "../../testdata",
		PRFiles:      protoregistry.GlobalFiles,
		Present:      true,
	}

	previous := gen.referredCache
	if err := previous.CheckRefer(context.Background(), prop, "1", input); err == nil {
		t.Fatal("first CheckRefer() error = nil, want load error")
	}
	previousCollector := gen.collector
	if err := gen.collector.Collect(errors.New("first run failed")); err != nil {
		t.Fatalf("collector reached its limit unexpectedly: %v", err)
	}

	gen.resetRunState()
	if gen.referredCache == previous {
		t.Fatal("resetRunState() reused the previous referred cache")
	}
	if err := gen.referredCache.CheckRefer(context.Background(), prop, "1", input); err == nil {
		t.Fatal("CheckRefer() after reset error = nil, want load to be retried")
	}
	if gen.collector == previousCollector {
		t.Fatal("resetRunState() reused the previous error collector")
	}
	if gen.collector.HasErrors() {
		t.Fatal("resetRunState() retained errors from the previous run")
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
