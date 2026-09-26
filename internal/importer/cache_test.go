package importer

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/tableauio/tableau/internal/importer/book"
	"google.golang.org/protobuf/proto"
)

type cacheTestParser struct{}

func (*cacheTestParser) Parse(proto.Message, *book.Sheet) error { return nil }

func TestCacheLoadSharesImporter(t *testing.T) {
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
			imp, err := cache.Load(context.Background(), "testdata/Test.xlsx", Sheets([]string{"Item"}))
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
}

func TestCacheLoadSeparatesSheetSelections(t *testing.T) {
	cache := NewCache()
	t.Cleanup(func() { _ = cache.Close() })
	items, err := cache.Load(context.Background(), "testdata/Test.xlsx", Sheets([]string{"Item"}))
	if err != nil {
		t.Fatal(err)
	}
	heroes, err := cache.Load(context.Background(), "testdata/Test.xlsx", Sheets([]string{"Hero"}))
	if err != nil {
		t.Fatal(err)
	}
	if items == heroes {
		t.Fatal("Load() shared importers with different sheet selections")
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
