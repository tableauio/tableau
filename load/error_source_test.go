package load_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/load"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/proto/tableaupb/unittestpb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

// Aliases and shard selectors describe the schema; cell locations identify the
// actual failing input for both merger and scatter loads.
func TestLoadShardSchemaContext(t *testing.T) {
	for _, mode := range []string{"Merger", "Scatter"} {
		t.Run(mode, func(t *testing.T) {
			fileOptions := &descriptorpb.FileOptions{}
			proto.SetExtension(fileOptions, tableaupb.E_Workbook,
				&tableaupb.WorkbookOptions{Name: "Primary#*.csv", Alias: "Tasks"})
			worksheet := &tableaupb.WorksheetOptions{Name: "Task", Namerow: 1, Typerow: 2, Noterow: 3, Datarow: 4}
			patterns := []string{"Shard*.csv#TaskSub"}
			if mode == "Merger" {
				worksheet.Merger = patterns
			} else {
				worksheet.Scatter = patterns
				worksheet.Patch = tableaupb.Patch_PATCH_REPLACE
			}
			messageOptions := &descriptorpb.MessageOptions{}
			proto.SetExtension(messageOptions, tableaupb.E_Worksheet, worksheet)
			fieldOptions := &descriptorpb.FieldOptions{}
			proto.SetExtension(fieldOptions, tableaupb.E_Field, &tableaupb.FieldOptions{Name: "Value"})
			fd, err := protodesc.NewFile(&descriptorpb.FileDescriptorProto{
				Name: proto.String("source.proto"), Syntax: proto.String("proto3"), Package: proto.String("loadtest"),
				Dependency: []string{"tableau/protobuf/tableau.proto"}, Options: fileOptions,
				MessageType: []*descriptorpb.DescriptorProto{{
					Name: proto.String("TaskConf"), Options: messageOptions,
					Field: []*descriptorpb.FieldDescriptorProto{{
						Name: proto.String("value"), Number: proto.Int32(1),
						Type: descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(), Options: fieldOptions,
					}},
				}},
			}, protoregistry.GlobalFiles)
			require.NoError(t, err)
			dir := t.TempDir()
			for filename, data := range map[string]string{
				"Primary#Task.csv":   "Value\nint32\nValue\n1\n",
				"Shard1#TaskSub.csv": "Value\nint32\nValue\ninvalid\n",
			} {
				require.NoError(t, os.WriteFile(filepath.Join(dir, filename), []byte(data), 0o600))
			}
			err = load.LoadMessagerInDir(dynamicpb.NewMessage(fd.Messages().Get(0)), dir, format.CSV, nil)
			require.Error(t, err)
			serr := tableau.Inspect(err)
			require.Len(t, serr.Details, 1)
			assert.Equal(t, "error[E2012]: invalid syntax of numerical value\nWorkbook: Shard1#*.csv (Primary: Primary#*.csv)\nWorksheet: TaskSub (Primary: Task)\nWorkbookAlias: Tasks\nWorksheetAlias: TaskConf\n"+mode+": Shard*.csv#TaskSub\nDataCellPos: A4\nDataCell: invalid\nReason: \"invalid\" cannot be parsed to numerical type \"int32\", strconv.ParseFloat: parsing \"invalid\": invalid syntax\nHelp: fill cell data with valid syntax of numerical type \"int32\"\n", serr.Error())
			source := serr.Details[0].Source
			require.NotNil(t, source)
			assert.Equal(t, "Shard1#*.csv", source.Workbook)
			assert.Equal(t, "Primary#*.csv", source.PrimaryWorkbook)
			assert.Equal(t, "TaskSub", source.Worksheet)
			assert.Equal(t, "Task", source.PrimaryWorksheet)
			assert.Equal(t, "Tasks", source.WorkbookAlias)
			assert.Equal(t, "TaskConf", source.WorksheetAlias)
			if mode == "Merger" {
				assert.Equal(t, patterns, source.Merger)
				assert.Empty(t, source.Scatter)
			} else {
				assert.Equal(t, patterns, source.Scatter)
				assert.Empty(t, source.Merger)
			}
			data, err := json.Marshal(serr)
			require.NoError(t, err)
			var decoded tableau.Error
			require.NoError(t, json.Unmarshal(data, &decoded))
			assert.Equal(t, source, decoded.Details[0].Source)
			assert.Equal(t, serr.Error(), decoded.Error())
		})
	}
}

func TestLoadMergerConflictSchemaContext(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "unittest"), 0o700))
	for _, filename := range []string{"Unittest#MergerSingleConf.csv", "UnittestMerger1#MergerSingleConf.csv"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "unittest", filename),
			[]byte("ID,Name\nuint32,string\nID,Name\n1,duplicate\n"), 0o600))
	}
	err := load.LoadMessagerInDir(&unittestpb.MergerSingleConf{}, dir, format.CSV, nil)
	require.Error(t, err)
	serr := tableau.Inspect(err)
	require.Len(t, serr.Details, 1)
	source := serr.Details[0].Source
	require.NotNil(t, source)
	assert.Contains(t, source.Workbook, "Unittest#*.csv")
	assert.Contains(t, source.Workbook, "UnittestMerger1#*.csv")
	assert.Equal(t, []string{"UnittestMerger1*.csv#MergerSingleConf"}, source.Merger)
	assert.Contains(t, serr.Error(), "Merger: UnittestMerger1*.csv#MergerSingleConf\n")
}
