package importer

import (
	"context"
	"sync"
	"testing"
)

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
	requests, imports, sheets, paths := cache.Stats()
	if requests != 2 || imports != 1 || sheets != 2 || paths != 1 {
		t.Fatalf("Stats() = (%d, %d, %d, %d), want (2, 1, 2, 1)", requests, imports, sheets, paths)
	}
}
