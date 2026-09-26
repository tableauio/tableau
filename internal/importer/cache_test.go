package importer

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/importer/book"
	"google.golang.org/protobuf/proto"
)

type cacheTestParser struct{}

func (*cacheTestParser) Parse(proto.Message, *book.Sheet) error { return nil }

var cacheOriginFormats = []struct {
	name           string
	format         format.Format
	firstFilename  string
	secondFilename string
	firstSheet     string
	secondSheet    string
}{
	{name: "excel", format: format.Excel, firstFilename: "testdata/Test.xlsx", secondFilename: "testdata/Test.xlsx", firstSheet: "Item", secondSheet: "Hero"},
	{name: "csv", format: format.CSV, firstFilename: "testdata/Test#Item.csv", secondFilename: "testdata/Test#Hero.csv", firstSheet: "Item", secondSheet: "Hero"},
	{name: "xml", format: format.XML, firstFilename: "testdata/Test.xml", secondFilename: "testdata/Test.xml", firstSheet: "@Item", secondSheet: "@TABLEAU"},
	{name: "yaml", format: format.YAML, firstFilename: "testdata/Test.yaml", secondFilename: "testdata/Test.yaml", firstSheet: "YamlScalarConf", secondSheet: "YamlStructConf"},
}

func TestCacheLoadSharesImporter(t *testing.T) {
	for _, tt := range cacheOriginFormats {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewCache()
			t.Cleanup(func() { _ = cache.Close() })
			const callers = 16
			results := make([]Importer, callers)
			start := make(chan struct{})
			var group sync.WaitGroup
			group.Add(callers)
			for i := range callers {
				go func(i int) {
					defer group.Done()
					<-start
					imp, err := cache.Load(context.Background(), tt.firstFilename, Sheets([]string{tt.firstSheet}))
					if err != nil {
						t.Errorf("Load() error = %v", err)
						return
					}
					results[i] = imp
				}(i)
			}
			close(start)
			group.Wait()

			for i := 1; i < callers; i++ {
				if results[i] != results[0] {
					t.Fatalf("Load() result %d was not shared", i)
				}
			}
		})
	}
}

func TestCacheMetricsDisabledByDefault(t *testing.T) {
	cache := NewCache()
	t.Cleanup(func() { _ = cache.Close() })
	if _, err := cache.Load(context.Background(), "testdata/Test.xlsx", Sheets([]string{"Item"})); err != nil {
		t.Fatal(err)
	}
	requests, imports, sheets, paths := cache.Metrics()
	if requests != 0 || imports != 0 || sheets != 0 || paths != 0 {
		t.Fatalf("Metrics() = (%d, %d, %d, %d), want all zero", requests, imports, sheets, paths)
	}
}

func TestCacheLoadSharesOriginFormats(t *testing.T) {
	covered := make(map[format.Format]bool, len(cacheOriginFormats))
	for _, tt := range cacheOriginFormats {
		covered[tt.format] = true
	}
	for _, originFormat := range format.InputFormats {
		if !covered[originFormat] {
			t.Fatalf("cache test does not cover origin format %q", originFormat)
		}
	}

	for _, tt := range cacheOriginFormats {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewCache()
			cache.EnableMetrics()
			t.Cleanup(func() { _ = cache.Close() })

			first, err := cache.Load(context.Background(), tt.firstFilename, Sheets([]string{tt.firstSheet}))
			if err != nil {
				t.Fatal(err)
			}
			all, err := cache.Load(context.Background(), tt.secondFilename, Sheets([]string{"*"}))
			if err != nil {
				t.Fatal(err)
			}
			if first == all {
				t.Fatal("Load() shared importers with different sheet selections")
			}
			if first.GetSheet(tt.firstSheet) == nil || first.GetSheet(tt.secondSheet) != nil {
				t.Fatalf("first view contains unexpected sheets: %v", first.GetSheets())
			}
			if all.GetSheet(tt.secondSheet) == nil {
				t.Fatalf("all-sheets view does not contain %q: %v", tt.secondSheet, all.GetSheets())
			}
			if first.GetSheet(tt.firstSheet) != all.GetSheet(tt.firstSheet) {
				t.Fatal("views do not share their decoded sheet")
			}
			requests, imports, sheets, paths := cache.Metrics()
			if requests != 2 || imports != 1 || sheets != int64(len(all.GetSheets())) || paths != 1 {
				t.Fatalf("Metrics() = (%d, %d, %d, %d), want (2, 1, %d, 1)", requests, imports, sheets, paths, len(all.GetSheets()))
			}
		})
	}
}

func TestCacheLoadBypassesCustomParser(t *testing.T) {
	cache := NewCache()
	t.Cleanup(func() { _ = cache.Close() })
	parser := &cacheTestParser{}
	first, err := cache.Load(context.Background(), "testdata/Test.xlsx", Sheets([]string{"Item"}), Parser(parser))
	if err != nil {
		t.Fatal(err)
	}
	second, err := cache.Load(context.Background(), "testdata/Test.xlsx", Sheets([]string{"Item"}), Parser(parser))
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("Load() cached an importer with a custom parser")
	}
}

func TestNilCacheLoadMergerImporters(t *testing.T) {
	var cache *Cache
	got, err := cache.LoadMergerImporters(
		context.Background(), ".", "testdata/Test.xml", "Item",
		[]string{"Test_*.xml"}, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("LoadMergerImporters() returned %d importers, want 1", len(got))
	}
}

func TestCacheLoadSharesClonedBook(t *testing.T) {
	cache := NewCache()
	cache.EnableMetrics()
	t.Cleanup(func() { _ = cache.Close() })
	items, err := cache.Load(context.Background(), "testdata/Test.xlsx", Sheets([]string{"Item"}), Cloned("testdata/Primary.xlsx"))
	if err != nil {
		t.Fatal(err)
	}
	heroes, err := cache.Load(context.Background(), "testdata/Test.xlsx", Sheets([]string{"Hero"}), Cloned("testdata/Primary.xlsx"))
	if err != nil {
		t.Fatal(err)
	}
	if items.GetSheet("Item") == nil || items.GetSheet("Hero") != nil {
		t.Fatal("Item view contains unexpected sheets")
	}
	if heroes.GetSheet("Hero") == nil || heroes.GetSheet("Item") != nil {
		t.Fatal("Hero view contains unexpected sheets")
	}
	requests, imports, sheets, paths := cache.Metrics()
	if requests != 2 || imports != 1 || sheets != 2 || paths != 1 {
		t.Fatalf("Metrics() = (%d, %d, %d, %d), want (2, 1, 2, 1)", requests, imports, sheets, paths)
	}
}

func TestCacheClose(t *testing.T) {
	cache := NewCache()
	if err := cache.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := cache.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
	if _, err := cache.Load(context.Background(), "testdata/Test.xlsx"); !errors.Is(err, errCacheClosed) {
		t.Fatalf("Load() error = %v, want %v", err, errCacheClosed)
	}
}
