package fieldprop

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/importer/book/tableparser"
	"github.com/tableauio/tableau/internal/x/xerrors"
	"github.com/tableauio/tableau/proto/tableaupb"
	_ "github.com/tableauio/tableau/proto/tableaupb/unittestpb"
	"google.golang.org/protobuf/reflect/protoregistry"
)

func Test_parseRefer(t *testing.T) {
	type args struct {
		text string
	}
	tests := []struct {
		name    string
		args    args
		want    *referDesc
		wantErr bool
	}{
		{
			name: "without alias",
			args: args{
				text: "Item.ID",
			},
			want: &referDesc{"Item", "", "ID"},
		},
		{
			name: "with alias",
			args: args{
				text: "Item(ItemConf).ID",
			},
			want: &referDesc{"Item", "ItemConf", "ID"},
		},
		{
			name: "special-sheet-name-and-with-alias",
			args: args{
				text: "Item-(Award)(ItemConf).ID",
			},
			want: &referDesc{"Item-(Award)", "ItemConf", "ID"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRefer(tt.args.text)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseRefer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseRefer() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInReferredSpace(t *testing.T) {
	type args struct {
		prop     *tableaupb.FieldProp
		cellData string
		input    *Input
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "in referred value space",
			args: args{
				prop: &tableaupb.FieldProp{
					Refer: "ItemConf.ID",
				},
				cellData: "1",
				input: &Input{
					ProtoPackage:   "unittest",
					InputDir:       "../../../testdata",
					SubdirRewrites: nil,
					PRFiles:        protoregistry.GlobalFiles,
					Present:        true,
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "not in referred value space",
			args: args{
				prop: &tableaupb.FieldProp{
					Refer: "ItemConf(ItemConf).ID",
				},
				cellData: "999",
				input: &Input{
					ProtoPackage:   "unittest",
					InputDir:       "../../../testdata",
					SubdirRewrites: nil,
					PRFiles:        protoregistry.GlobalFiles,
					Present:        true,
				},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "in ignored referred value space",
			args: args{
				prop: &tableaupb.FieldProp{
					Refer: "ItemConf(ItemConf).ID",
				},
				cellData: "4",
				input: &Input{
					ProtoPackage:   "unittest",
					InputDir:       "../../../testdata",
					SubdirRewrites: nil,
					PRFiles:        protoregistry.GlobalFiles,
					Present:        true,
				},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "in referred value space with subdir rewrites",
			args: args{
				prop: &tableaupb.FieldProp{
					Refer: "ItemConf(ItemConf).ID",
				},
				cellData: "1",
				input: &Input{
					ProtoPackage: "unittest",
					InputDir:     "../../../testdata/unittest",
					SubdirRewrites: map[string]string{
						"unittest/": "",
					},
					PRFiles: protoregistry.GlobalFiles,
					Present: true,
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "not in referred value space with subdir rewrites",
			args: args{
				prop: &tableaupb.FieldProp{
					Refer: "ItemConf(ItemConf).ID",
				},
				cellData: "999",
				input: &Input{
					ProtoPackage: "unittest",
					InputDir:     "../../../",
					SubdirRewrites: map[string]string{
						"unittest/": "testdata/unittest/",
					},
					PRFiles: protoregistry.GlobalFiles,
					Present: true,
				},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "in referred value space(transposed)",
			args: args{
				prop: &tableaupb.FieldProp{
					Refer: "Transpose.Name",
				},
				cellData: "Robin",
				input: &Input{
					ProtoPackage:   "unittest",
					InputDir:       "../../../testdata",
					SubdirRewrites: nil,
					PRFiles:        protoregistry.GlobalFiles,
					Present:        true,
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "not referred value space(transposed)",
			args: args{
				prop: &tableaupb.FieldProp{
					Refer: "Transpose.Name",
				},
				cellData: "Thomas",
				input: &Input{
					ProtoPackage:   "unittest",
					InputDir:       "../../../testdata",
					SubdirRewrites: nil,
					PRFiles:        protoregistry.GlobalFiles,
					Present:        true,
				},
			},
			want:    false,
			wantErr: false,
		},
	}
	cache := NewReferredCache()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := cache.InReferredSpace(context.Background(), tt.args.prop, tt.args.cellData, tt.args.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("InReferredSpace() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("InReferredSpace() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInReferredSpace_loadFailureDedup(t *testing.T) {
	cache := NewReferredCache()
	input := &Input{
		ProtoPackage:   "unittest",
		InputDir:       "../../../testdata",
		SubdirRewrites: nil,
		PRFiles:        protoregistry.GlobalFiles,
		Present:        true,
	}
	prop := &tableaupb.FieldProp{Refer: "DoesNotExistConf.ID"}

	ok, err := cache.InReferredSpace(context.Background(), prop, "1", input)
	if err == nil {
		t.Fatal("first InReferredSpace() error = nil, want load error")
	}
	if ok {
		t.Errorf("first InReferredSpace() = true, want false")
	}

	ok, err = cache.InReferredSpace(context.Background(), prop, "2", input)
	if err != nil {
		t.Fatalf("second InReferredSpace() error = %v, want nil (deduped)", err)
	}
	if !ok {
		t.Errorf("second InReferredSpace() = false, want true (treat as present after first failure)")
	}
}

func testHeader() *tableparser.Header {
	return &tableparser.Header{
		NameRow: 1,
		TypeRow: 2,
		NoteRow: 3,
		DataRow: 4,
	}
}

func TestValueSpace_AddFromTable(t *testing.T) {
	header := testHeader()

	t.Run("success", func(t *testing.T) {
		table := book.NewTable([][]string{
			{"ID", "Name"},
			{"uint32", "string"},
			{"id", "name"},
			{"1", "apple"},
			{"2", "orange"},
		})
		vs := newValueSpace()
		if err := vs.addFromTable(header, table, "ID", "Item.xlsx", "Item"); err != nil {
			t.Fatalf("AddFromTable() error = %v", err)
		}
		if !vs.Contains("1") || !vs.Contains("2") {
			t.Errorf("value space = %v, want {1, 2}", vs.Values())
		}
	})

	t.Run("missing column", func(t *testing.T) {
		table := book.NewTable([][]string{
			{"ID", "Name"},
			{"uint32", "string"},
			{"id", "name"},
			{"1", "apple"},
		})
		err := newValueSpace().addFromTable(header, table, "Missing", "AssistSkill.xlsx", "AssistSkill")
		if err == nil {
			t.Fatal("AddFromTable() error = nil, want error")
		}
		d := xerrors.NewDesc(err)
		if got := d.GetValue(xerrors.KeyReferBookName); got != "AssistSkill.xlsx" {
			t.Errorf("ReferBookName = %v, want AssistSkill.xlsx", got)
		}
		if got := d.GetValue(xerrors.KeyReferSheetName); got != "AssistSkill" {
			t.Errorf("ReferSheetName = %v, want AssistSkill", got)
		}
		if got := d.GetValue(xerrors.KeyBookName); got != nil {
			t.Errorf("BookName = %v, want nil", got)
		}
		if got := d.GetValue(xerrors.KeySheetName); got != nil {
			t.Errorf("SheetName = %v, want nil", got)
		}
	})

	t.Run("ignore parse error", func(t *testing.T) {
		table := book.NewTable([][]string{
			{"ID", "Name", "#IGNORE"},
			{"uint32", "string", "bool"},
			{"id", "name", "ignore"},
			{"1", "apple", "not-a-bool"},
		})
		err := newValueSpace().addFromTable(header, table, "ID", "AssistSkill.xlsx", "AssistSkill")
		if err == nil {
			t.Fatal("AddFromTable() error = nil, want error")
		}
		d := xerrors.NewDesc(err)
		if got := d.GetValue(xerrors.KeyReferBookName); got != "AssistSkill.xlsx" {
			t.Errorf("ReferBookName = %v, want AssistSkill.xlsx", got)
		}
		if got := d.GetValue(xerrors.KeyReferSheetName); got != "AssistSkill" {
			t.Errorf("ReferSheetName = %v, want AssistSkill", got)
		}
	})
}

func TestReferredCache_ExistsValue_loadFailureDedup(t *testing.T) {
	cache := NewReferredCache()
	var loads int32
	loadFunc := func() (*valueSpace, error) {
		atomic.AddInt32(&loads, 1)
		return nil, errors.New("load failed")
	}
	refer := "Broken.ID"

	ok, err := cache.existsValue(refer, "1", loadFunc)
	if err == nil {
		t.Fatal("first ExistsValue() error = nil, want error")
	}
	if ok {
		t.Errorf("first ExistsValue() = true, want false")
	}

	ok, err = cache.existsValue(refer, "2", loadFunc)
	if err != nil {
		t.Fatalf("second ExistsValue() error = %v, want nil", err)
	}
	if !ok {
		t.Errorf("second ExistsValue() = false, want true")
	}
	if got := atomic.LoadInt32(&loads); got != 1 {
		t.Errorf("loadFunc calls = %d, want 1", got)
	}
}

func TestReferredCache_ExistsValue_loadFailureDedupConcurrent(t *testing.T) {
	cache := NewReferredCache()
	var loads int32
	start := make(chan struct{})
	loadFunc := func() (*valueSpace, error) {
		atomic.AddInt32(&loads, 1)
		return nil, errors.New("load failed")
	}
	refer := "BrokenConcurrent.ID"

	const n = 32
	type result struct {
		ok  bool
		err error
	}
	results := make([]result, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			<-start
			ok, err := cache.existsValue(refer, "v", loadFunc)
			results[i] = result{ok, err}
		}(i)
	}
	close(start)
	wg.Wait()

	if got := atomic.LoadInt32(&loads); got != 1 {
		t.Errorf("loadFunc calls = %d, want 1", got)
	}
	var nErr, nOK int
	for _, res := range results {
		if res.err != nil {
			nErr++
			if res.ok {
				t.Errorf("failed ExistsValue() = true, want false")
			}
			continue
		}
		nOK++
		if !res.ok {
			t.Errorf("deduped ExistsValue() = false, want true")
		}
	}
	if nErr != 1 {
		t.Errorf("error returns = %d, want 1", nErr)
	}
	if nOK != n-1 {
		t.Errorf("deduped returns = %d, want %d", nOK, n-1)
	}
}

func TestInReferredSpace_nilReceiver(t *testing.T) {
	var cache *ReferredCache
	input := &Input{
		ProtoPackage: "unittest",
		InputDir:     "../../../testdata",
		PRFiles:      protoregistry.GlobalFiles,
		Present:      true,
	}
	_, err := cache.InReferredSpace(context.Background(), &tableaupb.FieldProp{Refer: "DoesNotExistConf.ID"}, "1", input)
	if err == nil {
		t.Fatal("nil receiver InReferredSpace() error = nil, want nil-cache error")
	}
	if got := err.Error(); got != "referred cache is nil" {
		t.Errorf("nil receiver InReferredSpace() error = %q, want %q", got, "referred cache is nil")
	}
}

func TestInReferredSpace_cacheIsolation(t *testing.T) {
	prop := &tableaupb.FieldProp{Refer: "DoesNotExistConf.ID"}
	input := &Input{
		ProtoPackage: "unittest",
		InputDir:     "../../../testdata",
		PRFiles:      protoregistry.GlobalFiles,
		Present:      true,
	}
	if _, err := NewReferredCache().InReferredSpace(context.Background(), prop, "1", input); err == nil {
		t.Fatal("first cache InReferredSpace() error = nil, want load error")
	}
	if _, err := NewReferredCache().InReferredSpace(context.Background(), prop, "1", input); err == nil {
		t.Fatal("isolated cache should retry load, want error")
	}
}

func TestLoadValueSpace_mergerErrorLocation(t *testing.T) {
	inputDir := t.TempDir()
	writeCSV := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(inputDir, name), []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	writeCSV("Unittest#MergerSingleConf.csv", "ID,Name\nuint32,string\nid,name\n1,main\n")
	writeCSV("UnittestMerger1#MergerSingleConf.csv", "Wrong,Name\nuint32,string\nwrong,name\n2,shard\n")

	_, err := loadValueSpace(context.Background(), "MergerSingleConf.ID", &Input{
		ProtoPackage: "unittest",
		InputDir:     inputDir,
		SubdirRewrites: map[string]string{
			"unittest/": "",
		},
		PRFiles: protoregistry.GlobalFiles,
		Present: true,
	})
	if err == nil {
		t.Fatal("loadValueSpace() error = nil, want missing-column error")
	}
	d := xerrors.NewDesc(err)
	if got := d.GetValue(xerrors.KeyReferBookName); got != "UnittestMerger1#*.csv" {
		t.Errorf("ReferBookName = %v, want UnittestMerger1#*.csv", got)
	}
	if got := d.GetValue(xerrors.KeyReferSheetName); got != "MergerSingleConf" {
		t.Errorf("ReferSheetName = %v, want MergerSingleConf", got)
	}
	if got := d.GetValue(xerrors.KeyBookName); got != nil {
		t.Errorf("BookName = %v, want nil", got)
	}
	if got := d.GetValue(xerrors.KeySheetName); got != nil {
		t.Errorf("SheetName = %v, want nil", got)
	}

	d = xerrors.NewDesc(xerrors.WrapKV(err,
		xerrors.KeyBookName, "Source#*.csv",
		xerrors.KeySheetName, "SourceConf",
	))
	if got := d.GetValue(xerrors.KeyBookName); got != "Source#*.csv" {
		t.Errorf("wrapped BookName = %v, want Source#*.csv", got)
	}
	if got := d.GetValue(xerrors.KeySheetName); got != "SourceConf" {
		t.Errorf("wrapped SheetName = %v, want SourceConf", got)
	}
}

func TestLoadValueSpace_preTableErrorLocations(t *testing.T) {
	t.Run("primary importer", func(t *testing.T) {
		_, err := loadValueSpace(context.Background(), "ItemConf.ID", &Input{
			ProtoPackage: "unittest",
			InputDir:     t.TempDir(),
			SubdirRewrites: map[string]string{
				"unittest/": "",
			},
			PRFiles: protoregistry.GlobalFiles,
			Present: true,
		})
		assertReferLocation(t, err, "unittest/Unittest#*.csv", "ItemConf")
	})

	t.Run("merger discovery", func(t *testing.T) {
		inputDir := t.TempDir()
		if err := os.WriteFile(
			filepath.Join(inputDir, "Unittest#MergerSingleConf.csv"),
			[]byte("ID,Name\nuint32,string\nid,name\n1,main\n"),
			0o600,
		); err != nil {
			t.Fatalf("write primary CSV: %v", err)
		}
		_, err := loadValueSpace(context.Background(), "MergerSingleConf.ID", &Input{
			ProtoPackage: "unittest",
			InputDir:     inputDir,
			SubdirRewrites: map[string]string{
				"unittest/": "",
			},
			PRFiles: protoregistry.GlobalFiles,
			Present: true,
		})
		assertReferLocation(t, err, "unittest/Unittest#*.csv", "MergerSingleConf")
	})

	t.Run("missing sheet", func(t *testing.T) {
		inputDir := t.TempDir()
		if err := os.WriteFile(
			filepath.Join(inputDir, "Unittest#Other.csv"),
			[]byte("ID\nuint32\nid\n1\n"),
			0o600,
		); err != nil {
			t.Fatalf("write unrelated CSV: %v", err)
		}
		_, err := loadValueSpace(context.Background(), "ItemConf.ID", &Input{
			ProtoPackage: "unittest",
			InputDir:     inputDir,
			SubdirRewrites: map[string]string{
				"unittest/": "",
			},
			PRFiles: protoregistry.GlobalFiles,
			Present: true,
		})
		assertReferLocation(t, err, "Unittest#*.csv", "ItemConf")
		if !errors.Is(err, xerrors.ErrE2030) {
			t.Errorf("loadValueSpace() error = %v, want E2030", err)
		}
	})
}

func assertReferLocation(t *testing.T, err error, wantBook, wantSheet string) {
	t.Helper()
	if err == nil {
		t.Fatal("loadValueSpace() error = nil")
	}
	d := xerrors.NewDesc(err)
	if got := d.GetValue(xerrors.KeyReferBookName); got != wantBook {
		t.Errorf("ReferBookName = %v, want %s", got, wantBook)
	}
	if got := d.GetValue(xerrors.KeyReferSheetName); got != wantSheet {
		t.Errorf("ReferSheetName = %v, want %s", got, wantSheet)
	}
	if got := d.GetValue(xerrors.KeyBookName); got != nil {
		t.Errorf("BookName = %v, want nil", got)
	}
	if got := d.GetValue(xerrors.KeySheetName); got != nil {
		t.Errorf("SheetName = %v, want nil", got)
	}
}
