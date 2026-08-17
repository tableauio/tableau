package fieldprop

import (
	"context"
	"errors"
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
	loadFunc := func(refer string) (*valueSpace, error) {
		atomic.AddInt32(&loads, 1)
		return nil, errors.New("load failed")
	}

	ok, err := cache.existsValue("Broken.ID", "1", loadFunc)
	if err == nil {
		t.Fatal("first ExistsValue() error = nil, want error")
	}
	if ok {
		t.Errorf("first ExistsValue() = true, want false")
	}

	ok, err = cache.existsValue("Broken.ID", "2", loadFunc)
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
	// All callers pass the RLock check before any one takes the write lock,
	// so losers hit the double-checked `failed` branch under Lock.
	cache := NewReferredCache()
	var loads int32
	start := make(chan struct{})
	loadFunc := func(refer string) (*valueSpace, error) {
		atomic.AddInt32(&loads, 1)
		return nil, errors.New("load failed")
	}

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
			ok, err := cache.existsValue("BrokenConcurrent.ID", "v", loadFunc)
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

func TestReferredCache_Reset(t *testing.T) {
	cache := NewReferredCache()
	vs := newValueSpace()
	vs.Add("1")
	cache.put("OK.ID", vs)

	var loads int32
	_, err := cache.existsValue("Broken.ID", "1", func(string) (*valueSpace, error) {
		atomic.AddInt32(&loads, 1)
		return nil, errors.New("load failed")
	})
	if err == nil {
		t.Fatal("ExistsValue() error = nil, want error")
	}

	cache.reset()

	if cache.exists("OK.ID") {
		t.Error("Exists(OK.ID) = true after Reset, want false")
	}

	_, err = cache.existsValue("Broken.ID", "1", func(string) (*valueSpace, error) {
		atomic.AddInt32(&loads, 1)
		return nil, errors.New("load failed")
	})
	if err == nil {
		t.Fatal("ExistsValue() after Reset error = nil, want load to be retried")
	}
	if got := atomic.LoadInt32(&loads); got != 2 {
		t.Errorf("loadFunc calls = %d, want 2 (retried after Reset)", got)
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
		t.Fatal("nil receiver InReferredSpace() error = nil, want load error")
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
