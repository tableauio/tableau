// Protoconf - Tableau's data interchange format
// https://tableauio.github.io/

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        v3.21.12
// source: tableau/protobuf/metabook.proto

package tableaupb

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type Metabook struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	MetasheetMap map[string]*Metasheet `protobuf:"bytes,1,rep,name=metasheet_map,json=metasheetMap,proto3" json:"metasheet_map,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
}

func (x *Metabook) Reset() {
	*x = Metabook{}
	if protoimpl.UnsafeEnabled {
		mi := &file_tableau_protobuf_metabook_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Metabook) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Metabook) ProtoMessage() {}

func (x *Metabook) ProtoReflect() protoreflect.Message {
	mi := &file_tableau_protobuf_metabook_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Metabook.ProtoReflect.Descriptor instead.
func (*Metabook) Descriptor() ([]byte, []int) {
	return file_tableau_protobuf_metabook_proto_rawDescGZIP(), []int{0}
}

func (x *Metabook) GetMetasheetMap() map[string]*Metasheet {
	if x != nil {
		return x.MetasheetMap
	}
	return nil
}

type Metasheet struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Sheet     string `protobuf:"bytes,1,opt,name=sheet,proto3" json:"sheet,omitempty"`
	Alias     string `protobuf:"bytes,2,opt,name=alias,proto3" json:"alias,omitempty"`
	Namerow   int32  `protobuf:"varint,3,opt,name=namerow,proto3" json:"namerow,omitempty"`
	Typerow   int32  `protobuf:"varint,4,opt,name=typerow,proto3" json:"typerow,omitempty"`
	Noterow   int32  `protobuf:"varint,5,opt,name=noterow,proto3" json:"noterow,omitempty"`
	Datarow   int32  `protobuf:"varint,6,opt,name=datarow,proto3" json:"datarow,omitempty"`
	Nameline  int32  `protobuf:"varint,7,opt,name=nameline,proto3" json:"nameline,omitempty"`
	Typeline  int32  `protobuf:"varint,8,opt,name=typeline,proto3" json:"typeline,omitempty"`
	Transpose bool   `protobuf:"varint,9,opt,name=transpose,proto3" json:"transpose,omitempty"`
	// nested naming of namerow
	Nested bool   `protobuf:"varint,10,opt,name=nested,proto3" json:"nested,omitempty"`
	Sep    string `protobuf:"bytes,11,opt,name=sep,proto3" json:"sep,omitempty"`
	Subsep string `protobuf:"bytes,12,opt,name=subsep,proto3" json:"subsep,omitempty"`
	// merge multiple sheets with same schema to one.
	// each element is:
	//   - a workbook name or Glob(https://pkg.go.dev/path/filepath#Glob) to merge (relative to this workbook): <Workbook>,
	//     then the sheet name is the same as this sheet.
	//   - or a workbook name (relative to this workbook) with a worksheet name: <Workbook>#<Worksheet>.
	Merger []string `protobuf:"bytes,13,rep,name=merger,proto3" json:"merger,omitempty"`
	// Tableau will merge adjacent rows with the same key. If the key cell is not set,
	// it will be treated as the same as the most nearest key above the same column.
	//
	// This option is only useful for map or keyed-list.
	AdjacentKey bool `protobuf:"varint,14,opt,name=adjacent_key,json=adjacentKey,proto3" json:"adjacent_key,omitempty"`
	// Field presence is the notion of whether a protobuf field has a value. If set as true,
	// in order to track presence for basic types (numeric, string, bytes, and enums), the
	// generated .proto will add the `optional` label to them.
	//
	// Singular proto3 fields of basic types (numeric, string, bytes, and enums) which are defined
	// with the optional label have explicit presence, like proto2 (this feature is enabled by default
	// as release 3.15). Refer: https://github.com/protocolbuffers/protobuf/blob/main/docs/field_presence.md
	FieldPresence bool `protobuf:"varint,15,opt,name=field_presence,json=fieldPresence,proto3" json:"field_presence,omitempty"`
	// declares if sheet is a template config, which only generates protobuf IDL and not generates json data.
	// NOTE: currently only used for XML protogen.
	Template bool `protobuf:"varint,16,opt,name=template,proto3" json:"template,omitempty"`
	// Sheet mode.
	Mode Mode `protobuf:"varint,17,opt,name=mode,proto3,enum=tableau.Mode" json:"mode,omitempty"`
	// Scatter converts sheets separately with same schema.
	// each element is:
	//   - a workbook name or Glob(https://pkg.go.dev/path/filepath#Glob) which is relative to this workbook: <Workbook>,
	//     then the sheet name is the same as this sheet.
	//   - or a workbook name which is relative to this workbook with a worksheet name: <Workbook>#<Worksheet>.
	Scatter []string `protobuf:"bytes,18,rep,name=scatter,proto3" json:"scatter,omitempty"`
	// Whether all fields in this sheet are optional (field name existence).
	// If set to true, then:
	//   - table formats (Excel/CSV): field's column can be absent.
	//   - document formats (XML/YAML): field's name can be absent.
	Optional bool `protobuf:"varint,19,opt,name=optional,proto3" json:"optional,omitempty"`
	// Sheet patch type.
	Patch Patch `protobuf:"varint,20,opt,name=patch,proto3,enum=tableau.Patch" json:"patch,omitempty"`
	// //////// Loader related options below //////////
	// Generate ordered map accessers
	OrderedMap bool `protobuf:"varint,50,opt,name=ordered_map,json=orderedMap,proto3" json:"ordered_map,omitempty"`
	// Generate index accessers, and multiple index columns are comma-separated.
	// Format: <ColumnName>[@IndexName], if IndexName is not set, it will be this
	// column’s parent struct type name.
	//
	// Composite indexes (or multicolumn indexes) are in the form: ([column1, column2, column3,...])[@IndexName]
	//
	// Examples:
	//   - ID
	//   - ID@Item
	//   - (ID,Type)
	//   - (ID,Type)@Item
	//   - ID, (ID,Type)@Item
	//
	// Generated APIs are:
	//
	// C++:
	// - const std::vector<const STRUCT_TYPE*>& Find<IndexName>(INDEX_TYPE index) const;
	// - const STRUCT_TYPE* FindFirst<IndexName>(INDEX_TYPE index);
	Index string `protobuf:"bytes,51,opt,name=index,proto3" json:"index,omitempty"`
}

func (x *Metasheet) Reset() {
	*x = Metasheet{}
	if protoimpl.UnsafeEnabled {
		mi := &file_tableau_protobuf_metabook_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Metasheet) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Metasheet) ProtoMessage() {}

func (x *Metasheet) ProtoReflect() protoreflect.Message {
	mi := &file_tableau_protobuf_metabook_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Metasheet.ProtoReflect.Descriptor instead.
func (*Metasheet) Descriptor() ([]byte, []int) {
	return file_tableau_protobuf_metabook_proto_rawDescGZIP(), []int{1}
}

func (x *Metasheet) GetSheet() string {
	if x != nil {
		return x.Sheet
	}
	return ""
}

func (x *Metasheet) GetAlias() string {
	if x != nil {
		return x.Alias
	}
	return ""
}

func (x *Metasheet) GetNamerow() int32 {
	if x != nil {
		return x.Namerow
	}
	return 0
}

func (x *Metasheet) GetTyperow() int32 {
	if x != nil {
		return x.Typerow
	}
	return 0
}

func (x *Metasheet) GetNoterow() int32 {
	if x != nil {
		return x.Noterow
	}
	return 0
}

func (x *Metasheet) GetDatarow() int32 {
	if x != nil {
		return x.Datarow
	}
	return 0
}

func (x *Metasheet) GetNameline() int32 {
	if x != nil {
		return x.Nameline
	}
	return 0
}

func (x *Metasheet) GetTypeline() int32 {
	if x != nil {
		return x.Typeline
	}
	return 0
}

func (x *Metasheet) GetTranspose() bool {
	if x != nil {
		return x.Transpose
	}
	return false
}

func (x *Metasheet) GetNested() bool {
	if x != nil {
		return x.Nested
	}
	return false
}

func (x *Metasheet) GetSep() string {
	if x != nil {
		return x.Sep
	}
	return ""
}

func (x *Metasheet) GetSubsep() string {
	if x != nil {
		return x.Subsep
	}
	return ""
}

func (x *Metasheet) GetMerger() []string {
	if x != nil {
		return x.Merger
	}
	return nil
}

func (x *Metasheet) GetAdjacentKey() bool {
	if x != nil {
		return x.AdjacentKey
	}
	return false
}

func (x *Metasheet) GetFieldPresence() bool {
	if x != nil {
		return x.FieldPresence
	}
	return false
}

func (x *Metasheet) GetTemplate() bool {
	if x != nil {
		return x.Template
	}
	return false
}

func (x *Metasheet) GetMode() Mode {
	if x != nil {
		return x.Mode
	}
	return Mode_MODE_DEFAULT
}

func (x *Metasheet) GetScatter() []string {
	if x != nil {
		return x.Scatter
	}
	return nil
}

func (x *Metasheet) GetOptional() bool {
	if x != nil {
		return x.Optional
	}
	return false
}

func (x *Metasheet) GetPatch() Patch {
	if x != nil {
		return x.Patch
	}
	return Patch_PATCH_NONE
}

func (x *Metasheet) GetOrderedMap() bool {
	if x != nil {
		return x.OrderedMap
	}
	return false
}

func (x *Metasheet) GetIndex() string {
	if x != nil {
		return x.Index
	}
	return ""
}

// EnumDescriptor represents enum type definition in sheet.
type EnumDescriptor struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Values []*EnumDescriptor_Value `protobuf:"bytes,1,rep,name=values,proto3" json:"values,omitempty"`
}

func (x *EnumDescriptor) Reset() {
	*x = EnumDescriptor{}
	if protoimpl.UnsafeEnabled {
		mi := &file_tableau_protobuf_metabook_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *EnumDescriptor) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EnumDescriptor) ProtoMessage() {}

func (x *EnumDescriptor) ProtoReflect() protoreflect.Message {
	mi := &file_tableau_protobuf_metabook_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EnumDescriptor.ProtoReflect.Descriptor instead.
func (*EnumDescriptor) Descriptor() ([]byte, []int) {
	return file_tableau_protobuf_metabook_proto_rawDescGZIP(), []int{2}
}

func (x *EnumDescriptor) GetValues() []*EnumDescriptor_Value {
	if x != nil {
		return x.Values
	}
	return nil
}

// StructDescriptor represents struct type definition in sheet.
type StructDescriptor struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Fields []*StructDescriptor_Field `protobuf:"bytes,1,rep,name=fields,proto3" json:"fields,omitempty"`
}

func (x *StructDescriptor) Reset() {
	*x = StructDescriptor{}
	if protoimpl.UnsafeEnabled {
		mi := &file_tableau_protobuf_metabook_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *StructDescriptor) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*StructDescriptor) ProtoMessage() {}

func (x *StructDescriptor) ProtoReflect() protoreflect.Message {
	mi := &file_tableau_protobuf_metabook_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use StructDescriptor.ProtoReflect.Descriptor instead.
func (*StructDescriptor) Descriptor() ([]byte, []int) {
	return file_tableau_protobuf_metabook_proto_rawDescGZIP(), []int{3}
}

func (x *StructDescriptor) GetFields() []*StructDescriptor_Field {
	if x != nil {
		return x.Fields
	}
	return nil
}

// UnionDescriptor represents union type definition in sheet.
type UnionDescriptor struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Values []*UnionDescriptor_Value `protobuf:"bytes,1,rep,name=values,proto3" json:"values,omitempty"`
}

func (x *UnionDescriptor) Reset() {
	*x = UnionDescriptor{}
	if protoimpl.UnsafeEnabled {
		mi := &file_tableau_protobuf_metabook_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UnionDescriptor) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UnionDescriptor) ProtoMessage() {}

func (x *UnionDescriptor) ProtoReflect() protoreflect.Message {
	mi := &file_tableau_protobuf_metabook_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UnionDescriptor.ProtoReflect.Descriptor instead.
func (*UnionDescriptor) Descriptor() ([]byte, []int) {
	return file_tableau_protobuf_metabook_proto_rawDescGZIP(), []int{4}
}

func (x *UnionDescriptor) GetValues() []*UnionDescriptor_Value {
	if x != nil {
		return x.Values
	}
	return nil
}

type EnumDescriptor_Value struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Number *int32 `protobuf:"varint,1,opt,name=number,proto3,oneof" json:"number,omitempty"`
	Name   string `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
	Alias  string `protobuf:"bytes,3,opt,name=alias,proto3" json:"alias,omitempty"`
}

func (x *EnumDescriptor_Value) Reset() {
	*x = EnumDescriptor_Value{}
	if protoimpl.UnsafeEnabled {
		mi := &file_tableau_protobuf_metabook_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *EnumDescriptor_Value) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EnumDescriptor_Value) ProtoMessage() {}

func (x *EnumDescriptor_Value) ProtoReflect() protoreflect.Message {
	mi := &file_tableau_protobuf_metabook_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EnumDescriptor_Value.ProtoReflect.Descriptor instead.
func (*EnumDescriptor_Value) Descriptor() ([]byte, []int) {
	return file_tableau_protobuf_metabook_proto_rawDescGZIP(), []int{2, 0}
}

func (x *EnumDescriptor_Value) GetNumber() int32 {
	if x != nil && x.Number != nil {
		return *x.Number
	}
	return 0
}

func (x *EnumDescriptor_Value) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *EnumDescriptor_Value) GetAlias() string {
	if x != nil {
		return x.Alias
	}
	return ""
}

type StructDescriptor_Field struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Type string `protobuf:"bytes,2,opt,name=type,proto3" json:"type,omitempty"`
}

func (x *StructDescriptor_Field) Reset() {
	*x = StructDescriptor_Field{}
	if protoimpl.UnsafeEnabled {
		mi := &file_tableau_protobuf_metabook_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *StructDescriptor_Field) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*StructDescriptor_Field) ProtoMessage() {}

func (x *StructDescriptor_Field) ProtoReflect() protoreflect.Message {
	mi := &file_tableau_protobuf_metabook_proto_msgTypes[7]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use StructDescriptor_Field.ProtoReflect.Descriptor instead.
func (*StructDescriptor_Field) Descriptor() ([]byte, []int) {
	return file_tableau_protobuf_metabook_proto_rawDescGZIP(), []int{3, 0}
}

func (x *StructDescriptor_Field) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *StructDescriptor_Field) GetType() string {
	if x != nil {
		return x.Type
	}
	return ""
}

type UnionDescriptor_Value struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Number *int32 `protobuf:"varint,1,opt,name=number,proto3,oneof" json:"number,omitempty"`
	// This is message type name, and the corresponding enum value name
	// is generated as: "TYPE_" + strcase.ToScreamingSnake(name).
	Name   string   `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
	Alias  string   `protobuf:"bytes,3,opt,name=alias,proto3" json:"alias,omitempty"`
	Fields []string `protobuf:"bytes,4,rep,name=fields,proto3" json:"fields,omitempty"`
}

func (x *UnionDescriptor_Value) Reset() {
	*x = UnionDescriptor_Value{}
	if protoimpl.UnsafeEnabled {
		mi := &file_tableau_protobuf_metabook_proto_msgTypes[8]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UnionDescriptor_Value) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UnionDescriptor_Value) ProtoMessage() {}

func (x *UnionDescriptor_Value) ProtoReflect() protoreflect.Message {
	mi := &file_tableau_protobuf_metabook_proto_msgTypes[8]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UnionDescriptor_Value.ProtoReflect.Descriptor instead.
func (*UnionDescriptor_Value) Descriptor() ([]byte, []int) {
	return file_tableau_protobuf_metabook_proto_rawDescGZIP(), []int{4, 0}
}

func (x *UnionDescriptor_Value) GetNumber() int32 {
	if x != nil && x.Number != nil {
		return *x.Number
	}
	return 0
}

func (x *UnionDescriptor_Value) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *UnionDescriptor_Value) GetAlias() string {
	if x != nil {
		return x.Alias
	}
	return ""
}

func (x *UnionDescriptor_Value) GetFields() []string {
	if x != nil {
		return x.Fields
	}
	return nil
}

var File_tableau_protobuf_metabook_proto protoreflect.FileDescriptor

var file_tableau_protobuf_metabook_proto_rawDesc = []byte{
	0x0a, 0x1f, 0x74, 0x61, 0x62, 0x6c, 0x65, 0x61, 0x75, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62,
	0x75, 0x66, 0x2f, 0x6d, 0x65, 0x74, 0x61, 0x62, 0x6f, 0x6f, 0x6b, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x12, 0x07, 0x74, 0x61, 0x62, 0x6c, 0x65, 0x61, 0x75, 0x1a, 0x1e, 0x74, 0x61, 0x62, 0x6c,
	0x65, 0x61, 0x75, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x74, 0x61, 0x62,
	0x6c, 0x65, 0x61, 0x75, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0xca, 0x01, 0x0a, 0x08, 0x4d,
	0x65, 0x74, 0x61, 0x62, 0x6f, 0x6f, 0x6b, 0x12, 0x55, 0x0a, 0x0d, 0x6d, 0x65, 0x74, 0x61, 0x73,
	0x68, 0x65, 0x65, 0x74, 0x5f, 0x6d, 0x61, 0x70, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x23,
	0x2e, 0x74, 0x61, 0x62, 0x6c, 0x65, 0x61, 0x75, 0x2e, 0x4d, 0x65, 0x74, 0x61, 0x62, 0x6f, 0x6f,
	0x6b, 0x2e, 0x4d, 0x65, 0x74, 0x61, 0x73, 0x68, 0x65, 0x65, 0x74, 0x4d, 0x61, 0x70, 0x45, 0x6e,
	0x74, 0x72, 0x79, 0x42, 0x0b, 0x82, 0xb5, 0x18, 0x07, 0x1a, 0x05, 0x53, 0x68, 0x65, 0x65, 0x74,
	0x52, 0x0c, 0x6d, 0x65, 0x74, 0x61, 0x73, 0x68, 0x65, 0x65, 0x74, 0x4d, 0x61, 0x70, 0x1a, 0x53,
	0x0a, 0x11, 0x4d, 0x65, 0x74, 0x61, 0x73, 0x68, 0x65, 0x65, 0x74, 0x4d, 0x61, 0x70, 0x45, 0x6e,
	0x74, 0x72, 0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x28, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x0b, 0x32, 0x12, 0x2e, 0x74, 0x61, 0x62, 0x6c, 0x65, 0x61, 0x75, 0x2e, 0x4d,
	0x65, 0x74, 0x61, 0x73, 0x68, 0x65, 0x65, 0x74, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a,
	0x02, 0x38, 0x01, 0x3a, 0x12, 0x82, 0xb5, 0x18, 0x0e, 0x0a, 0x08, 0x40, 0x54, 0x41, 0x42, 0x4c,
	0x45, 0x41, 0x55, 0x10, 0x01, 0x28, 0x02, 0x22, 0x8e, 0x08, 0x0a, 0x09, 0x4d, 0x65, 0x74, 0x61,
	0x73, 0x68, 0x65, 0x65, 0x74, 0x12, 0x21, 0x0a, 0x05, 0x73, 0x68, 0x65, 0x65, 0x74, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x09, 0x42, 0x0b, 0x82, 0xb5, 0x18, 0x07, 0x0a, 0x05, 0x53, 0x68, 0x65, 0x65,
	0x74, 0x52, 0x05, 0x73, 0x68, 0x65, 0x65, 0x74, 0x12, 0x25, 0x0a, 0x05, 0x61, 0x6c, 0x69, 0x61,
	0x73, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x42, 0x0f, 0x82, 0xb5, 0x18, 0x0b, 0x0a, 0x05, 0x41,
	0x6c, 0x69, 0x61, 0x73, 0x7a, 0x02, 0x58, 0x01, 0x52, 0x05, 0x61, 0x6c, 0x69, 0x61, 0x73, 0x12,
	0x2b, 0x0a, 0x07, 0x6e, 0x61, 0x6d, 0x65, 0x72, 0x6f, 0x77, 0x18, 0x03, 0x20, 0x01, 0x28, 0x05,
	0x42, 0x11, 0x82, 0xb5, 0x18, 0x0d, 0x0a, 0x07, 0x4e, 0x61, 0x6d, 0x65, 0x72, 0x6f, 0x77, 0x7a,
	0x02, 0x58, 0x01, 0x52, 0x07, 0x6e, 0x61, 0x6d, 0x65, 0x72, 0x6f, 0x77, 0x12, 0x2b, 0x0a, 0x07,
	0x74, 0x79, 0x70, 0x65, 0x72, 0x6f, 0x77, 0x18, 0x04, 0x20, 0x01, 0x28, 0x05, 0x42, 0x11, 0x82,
	0xb5, 0x18, 0x0d, 0x0a, 0x07, 0x54, 0x79, 0x70, 0x65, 0x72, 0x6f, 0x77, 0x7a, 0x02, 0x58, 0x01,
	0x52, 0x07, 0x74, 0x79, 0x70, 0x65, 0x72, 0x6f, 0x77, 0x12, 0x2b, 0x0a, 0x07, 0x6e, 0x6f, 0x74,
	0x65, 0x72, 0x6f, 0x77, 0x18, 0x05, 0x20, 0x01, 0x28, 0x05, 0x42, 0x11, 0x82, 0xb5, 0x18, 0x0d,
	0x0a, 0x07, 0x4e, 0x6f, 0x74, 0x65, 0x72, 0x6f, 0x77, 0x7a, 0x02, 0x58, 0x01, 0x52, 0x07, 0x6e,
	0x6f, 0x74, 0x65, 0x72, 0x6f, 0x77, 0x12, 0x2b, 0x0a, 0x07, 0x64, 0x61, 0x74, 0x61, 0x72, 0x6f,
	0x77, 0x18, 0x06, 0x20, 0x01, 0x28, 0x05, 0x42, 0x11, 0x82, 0xb5, 0x18, 0x0d, 0x0a, 0x07, 0x44,
	0x61, 0x74, 0x61, 0x72, 0x6f, 0x77, 0x7a, 0x02, 0x58, 0x01, 0x52, 0x07, 0x64, 0x61, 0x74, 0x61,
	0x72, 0x6f, 0x77, 0x12, 0x2e, 0x0a, 0x08, 0x6e, 0x61, 0x6d, 0x65, 0x6c, 0x69, 0x6e, 0x65, 0x18,
	0x07, 0x20, 0x01, 0x28, 0x05, 0x42, 0x12, 0x82, 0xb5, 0x18, 0x0e, 0x0a, 0x08, 0x4e, 0x61, 0x6d,
	0x65, 0x6c, 0x69, 0x6e, 0x65, 0x7a, 0x02, 0x58, 0x01, 0x52, 0x08, 0x6e, 0x61, 0x6d, 0x65, 0x6c,
	0x69, 0x6e, 0x65, 0x12, 0x2e, 0x0a, 0x08, 0x74, 0x79, 0x70, 0x65, 0x6c, 0x69, 0x6e, 0x65, 0x18,
	0x08, 0x20, 0x01, 0x28, 0x05, 0x42, 0x12, 0x82, 0xb5, 0x18, 0x0e, 0x0a, 0x08, 0x54, 0x79, 0x70,
	0x65, 0x6c, 0x69, 0x6e, 0x65, 0x7a, 0x02, 0x58, 0x01, 0x52, 0x08, 0x74, 0x79, 0x70, 0x65, 0x6c,
	0x69, 0x6e, 0x65, 0x12, 0x31, 0x0a, 0x09, 0x74, 0x72, 0x61, 0x6e, 0x73, 0x70, 0x6f, 0x73, 0x65,
	0x18, 0x09, 0x20, 0x01, 0x28, 0x08, 0x42, 0x13, 0x82, 0xb5, 0x18, 0x0f, 0x0a, 0x09, 0x54, 0x72,
	0x61, 0x6e, 0x73, 0x70, 0x6f, 0x73, 0x65, 0x7a, 0x02, 0x58, 0x01, 0x52, 0x09, 0x74, 0x72, 0x61,
	0x6e, 0x73, 0x70, 0x6f, 0x73, 0x65, 0x12, 0x28, 0x0a, 0x06, 0x6e, 0x65, 0x73, 0x74, 0x65, 0x64,
	0x18, 0x0a, 0x20, 0x01, 0x28, 0x08, 0x42, 0x10, 0x82, 0xb5, 0x18, 0x0c, 0x0a, 0x06, 0x4e, 0x65,
	0x73, 0x74, 0x65, 0x64, 0x7a, 0x02, 0x58, 0x01, 0x52, 0x06, 0x6e, 0x65, 0x73, 0x74, 0x65, 0x64,
	0x12, 0x1f, 0x0a, 0x03, 0x73, 0x65, 0x70, 0x18, 0x0b, 0x20, 0x01, 0x28, 0x09, 0x42, 0x0d, 0x82,
	0xb5, 0x18, 0x09, 0x0a, 0x03, 0x53, 0x65, 0x70, 0x7a, 0x02, 0x58, 0x01, 0x52, 0x03, 0x73, 0x65,
	0x70, 0x12, 0x28, 0x0a, 0x06, 0x73, 0x75, 0x62, 0x73, 0x65, 0x70, 0x18, 0x0c, 0x20, 0x01, 0x28,
	0x09, 0x42, 0x10, 0x82, 0xb5, 0x18, 0x0c, 0x0a, 0x06, 0x53, 0x75, 0x62, 0x73, 0x65, 0x70, 0x7a,
	0x02, 0x58, 0x01, 0x52, 0x06, 0x73, 0x75, 0x62, 0x73, 0x65, 0x70, 0x12, 0x2a, 0x0a, 0x06, 0x6d,
	0x65, 0x72, 0x67, 0x65, 0x72, 0x18, 0x0d, 0x20, 0x03, 0x28, 0x09, 0x42, 0x12, 0x82, 0xb5, 0x18,
	0x0e, 0x0a, 0x06, 0x4d, 0x65, 0x72, 0x67, 0x65, 0x72, 0x20, 0x03, 0x7a, 0x02, 0x58, 0x01, 0x52,
	0x06, 0x6d, 0x65, 0x72, 0x67, 0x65, 0x72, 0x12, 0x38, 0x0a, 0x0c, 0x61, 0x64, 0x6a, 0x61, 0x63,
	0x65, 0x6e, 0x74, 0x5f, 0x6b, 0x65, 0x79, 0x18, 0x0e, 0x20, 0x01, 0x28, 0x08, 0x42, 0x15, 0x82,
	0xb5, 0x18, 0x11, 0x0a, 0x0b, 0x41, 0x64, 0x6a, 0x61, 0x63, 0x65, 0x6e, 0x74, 0x4b, 0x65, 0x79,
	0x7a, 0x02, 0x58, 0x01, 0x52, 0x0b, 0x61, 0x64, 0x6a, 0x61, 0x63, 0x65, 0x6e, 0x74, 0x4b, 0x65,
	0x79, 0x12, 0x3e, 0x0a, 0x0e, 0x66, 0x69, 0x65, 0x6c, 0x64, 0x5f, 0x70, 0x72, 0x65, 0x73, 0x65,
	0x6e, 0x63, 0x65, 0x18, 0x0f, 0x20, 0x01, 0x28, 0x08, 0x42, 0x17, 0x82, 0xb5, 0x18, 0x13, 0x0a,
	0x0d, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x50, 0x72, 0x65, 0x73, 0x65, 0x6e, 0x63, 0x65, 0x7a, 0x02,
	0x58, 0x01, 0x52, 0x0d, 0x66, 0x69, 0x65, 0x6c, 0x64, 0x50, 0x72, 0x65, 0x73, 0x65, 0x6e, 0x63,
	0x65, 0x12, 0x2e, 0x0a, 0x08, 0x74, 0x65, 0x6d, 0x70, 0x6c, 0x61, 0x74, 0x65, 0x18, 0x10, 0x20,
	0x01, 0x28, 0x08, 0x42, 0x12, 0x82, 0xb5, 0x18, 0x0e, 0x0a, 0x08, 0x54, 0x65, 0x6d, 0x70, 0x6c,
	0x61, 0x74, 0x65, 0x7a, 0x02, 0x58, 0x01, 0x52, 0x08, 0x74, 0x65, 0x6d, 0x70, 0x6c, 0x61, 0x74,
	0x65, 0x12, 0x31, 0x0a, 0x04, 0x6d, 0x6f, 0x64, 0x65, 0x18, 0x11, 0x20, 0x01, 0x28, 0x0e, 0x32,
	0x0d, 0x2e, 0x74, 0x61, 0x62, 0x6c, 0x65, 0x61, 0x75, 0x2e, 0x4d, 0x6f, 0x64, 0x65, 0x42, 0x0e,
	0x82, 0xb5, 0x18, 0x0a, 0x0a, 0x04, 0x4d, 0x6f, 0x64, 0x65, 0x7a, 0x02, 0x58, 0x01, 0x52, 0x04,
	0x6d, 0x6f, 0x64, 0x65, 0x12, 0x2d, 0x0a, 0x07, 0x73, 0x63, 0x61, 0x74, 0x74, 0x65, 0x72, 0x18,
	0x12, 0x20, 0x03, 0x28, 0x09, 0x42, 0x13, 0x82, 0xb5, 0x18, 0x0f, 0x0a, 0x07, 0x53, 0x63, 0x61,
	0x74, 0x74, 0x65, 0x72, 0x20, 0x03, 0x7a, 0x02, 0x58, 0x01, 0x52, 0x07, 0x73, 0x63, 0x61, 0x74,
	0x74, 0x65, 0x72, 0x12, 0x2e, 0x0a, 0x08, 0x6f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x61, 0x6c, 0x18,
	0x13, 0x20, 0x01, 0x28, 0x08, 0x42, 0x12, 0x82, 0xb5, 0x18, 0x0e, 0x0a, 0x08, 0x4f, 0x70, 0x74,
	0x69, 0x6f, 0x6e, 0x61, 0x6c, 0x7a, 0x02, 0x58, 0x01, 0x52, 0x08, 0x6f, 0x70, 0x74, 0x69, 0x6f,
	0x6e, 0x61, 0x6c, 0x12, 0x35, 0x0a, 0x05, 0x70, 0x61, 0x74, 0x63, 0x68, 0x18, 0x14, 0x20, 0x01,
	0x28, 0x0e, 0x32, 0x0e, 0x2e, 0x74, 0x61, 0x62, 0x6c, 0x65, 0x61, 0x75, 0x2e, 0x50, 0x61, 0x74,
	0x63, 0x68, 0x42, 0x0f, 0x82, 0xb5, 0x18, 0x0b, 0x0a, 0x05, 0x50, 0x61, 0x74, 0x63, 0x68, 0x7a,
	0x02, 0x58, 0x01, 0x52, 0x05, 0x70, 0x61, 0x74, 0x63, 0x68, 0x12, 0x35, 0x0a, 0x0b, 0x6f, 0x72,
	0x64, 0x65, 0x72, 0x65, 0x64, 0x5f, 0x6d, 0x61, 0x70, 0x18, 0x32, 0x20, 0x01, 0x28, 0x08, 0x42,
	0x14, 0x82, 0xb5, 0x18, 0x10, 0x0a, 0x0a, 0x4f, 0x72, 0x64, 0x65, 0x72, 0x65, 0x64, 0x4d, 0x61,
	0x70, 0x7a, 0x02, 0x58, 0x01, 0x52, 0x0a, 0x6f, 0x72, 0x64, 0x65, 0x72, 0x65, 0x64, 0x4d, 0x61,
	0x70, 0x12, 0x25, 0x0a, 0x05, 0x69, 0x6e, 0x64, 0x65, 0x78, 0x18, 0x33, 0x20, 0x01, 0x28, 0x09,
	0x42, 0x0f, 0x82, 0xb5, 0x18, 0x0b, 0x0a, 0x05, 0x49, 0x6e, 0x64, 0x65, 0x78, 0x7a, 0x02, 0x58,
	0x01, 0x52, 0x05, 0x69, 0x6e, 0x64, 0x65, 0x78, 0x22, 0xe0, 0x01, 0x0a, 0x0e, 0x45, 0x6e, 0x75,
	0x6d, 0x44, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x6f, 0x72, 0x12, 0x3d, 0x0a, 0x06, 0x76,
	0x61, 0x6c, 0x75, 0x65, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x1d, 0x2e, 0x74, 0x61,
	0x62, 0x6c, 0x65, 0x61, 0x75, 0x2e, 0x45, 0x6e, 0x75, 0x6d, 0x44, 0x65, 0x73, 0x63, 0x72, 0x69,
	0x70, 0x74, 0x6f, 0x72, 0x2e, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x42, 0x06, 0x82, 0xb5, 0x18, 0x02,
	0x20, 0x01, 0x52, 0x06, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x73, 0x1a, 0x84, 0x01, 0x0a, 0x05, 0x56,
	0x61, 0x6c, 0x75, 0x65, 0x12, 0x2d, 0x0a, 0x06, 0x6e, 0x75, 0x6d, 0x62, 0x65, 0x72, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x05, 0x42, 0x10, 0x82, 0xb5, 0x18, 0x0c, 0x0a, 0x06, 0x4e, 0x75, 0x6d, 0x62,
	0x65, 0x72, 0x7a, 0x02, 0x58, 0x01, 0x48, 0x00, 0x52, 0x06, 0x6e, 0x75, 0x6d, 0x62, 0x65, 0x72,
	0x88, 0x01, 0x01, 0x12, 0x1e, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x09, 0x42, 0x0a, 0x82, 0xb5, 0x18, 0x06, 0x0a, 0x04, 0x4e, 0x61, 0x6d, 0x65, 0x52, 0x04, 0x6e,
	0x61, 0x6d, 0x65, 0x12, 0x21, 0x0a, 0x05, 0x61, 0x6c, 0x69, 0x61, 0x73, 0x18, 0x03, 0x20, 0x01,
	0x28, 0x09, 0x42, 0x0b, 0x82, 0xb5, 0x18, 0x07, 0x0a, 0x05, 0x41, 0x6c, 0x69, 0x61, 0x73, 0x52,
	0x05, 0x61, 0x6c, 0x69, 0x61, 0x73, 0x42, 0x09, 0x0a, 0x07, 0x5f, 0x6e, 0x75, 0x6d, 0x62, 0x65,
	0x72, 0x3a, 0x08, 0x82, 0xb5, 0x18, 0x04, 0x10, 0x01, 0x28, 0x02, 0x22, 0xa6, 0x01, 0x0a, 0x10,
	0x53, 0x74, 0x72, 0x75, 0x63, 0x74, 0x44, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x6f, 0x72,
	0x12, 0x3f, 0x0a, 0x06, 0x66, 0x69, 0x65, 0x6c, 0x64, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b,
	0x32, 0x1f, 0x2e, 0x74, 0x61, 0x62, 0x6c, 0x65, 0x61, 0x75, 0x2e, 0x53, 0x74, 0x72, 0x75, 0x63,
	0x74, 0x44, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x6f, 0x72, 0x2e, 0x46, 0x69, 0x65, 0x6c,
	0x64, 0x42, 0x06, 0x82, 0xb5, 0x18, 0x02, 0x20, 0x01, 0x52, 0x06, 0x66, 0x69, 0x65, 0x6c, 0x64,
	0x73, 0x1a, 0x47, 0x0a, 0x05, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x12, 0x1e, 0x0a, 0x04, 0x6e, 0x61,
	0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x42, 0x0a, 0x82, 0xb5, 0x18, 0x06, 0x0a, 0x04,
	0x4e, 0x61, 0x6d, 0x65, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x1e, 0x0a, 0x04, 0x74, 0x79,
	0x70, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x42, 0x0a, 0x82, 0xb5, 0x18, 0x06, 0x0a, 0x04,
	0x54, 0x79, 0x70, 0x65, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65, 0x3a, 0x08, 0x82, 0xb5, 0x18, 0x04,
	0x10, 0x01, 0x28, 0x02, 0x22, 0x89, 0x02, 0x0a, 0x0f, 0x55, 0x6e, 0x69, 0x6f, 0x6e, 0x44, 0x65,
	0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x6f, 0x72, 0x12, 0x3e, 0x0a, 0x06, 0x76, 0x61, 0x6c, 0x75,
	0x65, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x1e, 0x2e, 0x74, 0x61, 0x62, 0x6c, 0x65,
	0x61, 0x75, 0x2e, 0x55, 0x6e, 0x69, 0x6f, 0x6e, 0x44, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74,
	0x6f, 0x72, 0x2e, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x42, 0x06, 0x82, 0xb5, 0x18, 0x02, 0x20, 0x01,
	0x52, 0x06, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x73, 0x1a, 0xab, 0x01, 0x0a, 0x05, 0x56, 0x61, 0x6c,
	0x75, 0x65, 0x12, 0x2d, 0x0a, 0x06, 0x6e, 0x75, 0x6d, 0x62, 0x65, 0x72, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x05, 0x42, 0x10, 0x82, 0xb5, 0x18, 0x0c, 0x0a, 0x06, 0x4e, 0x75, 0x6d, 0x62, 0x65, 0x72,
	0x7a, 0x02, 0x58, 0x01, 0x48, 0x00, 0x52, 0x06, 0x6e, 0x75, 0x6d, 0x62, 0x65, 0x72, 0x88, 0x01,
	0x01, 0x12, 0x1e, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x42,
	0x0a, 0x82, 0xb5, 0x18, 0x06, 0x0a, 0x04, 0x4e, 0x61, 0x6d, 0x65, 0x52, 0x04, 0x6e, 0x61, 0x6d,
	0x65, 0x12, 0x21, 0x0a, 0x05, 0x61, 0x6c, 0x69, 0x61, 0x73, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09,
	0x42, 0x0b, 0x82, 0xb5, 0x18, 0x07, 0x0a, 0x05, 0x41, 0x6c, 0x69, 0x61, 0x73, 0x52, 0x05, 0x61,
	0x6c, 0x69, 0x61, 0x73, 0x12, 0x25, 0x0a, 0x06, 0x66, 0x69, 0x65, 0x6c, 0x64, 0x73, 0x18, 0x04,
	0x20, 0x03, 0x28, 0x09, 0x42, 0x0d, 0x82, 0xb5, 0x18, 0x09, 0x0a, 0x05, 0x46, 0x69, 0x65, 0x6c,
	0x64, 0x20, 0x02, 0x52, 0x06, 0x66, 0x69, 0x65, 0x6c, 0x64, 0x73, 0x42, 0x09, 0x0a, 0x07, 0x5f,
	0x6e, 0x75, 0x6d, 0x62, 0x65, 0x72, 0x3a, 0x08, 0x82, 0xb5, 0x18, 0x04, 0x10, 0x01, 0x28, 0x02,
	0x42, 0x2e, 0x5a, 0x2c, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x74,
	0x61, 0x62, 0x6c, 0x65, 0x61, 0x75, 0x69, 0x6f, 0x2f, 0x74, 0x61, 0x62, 0x6c, 0x65, 0x61, 0x75,
	0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x74, 0x61, 0x62, 0x6c, 0x65, 0x61, 0x75, 0x70, 0x62,
	0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_tableau_protobuf_metabook_proto_rawDescOnce sync.Once
	file_tableau_protobuf_metabook_proto_rawDescData = file_tableau_protobuf_metabook_proto_rawDesc
)

func file_tableau_protobuf_metabook_proto_rawDescGZIP() []byte {
	file_tableau_protobuf_metabook_proto_rawDescOnce.Do(func() {
		file_tableau_protobuf_metabook_proto_rawDescData = protoimpl.X.CompressGZIP(file_tableau_protobuf_metabook_proto_rawDescData)
	})
	return file_tableau_protobuf_metabook_proto_rawDescData
}

var file_tableau_protobuf_metabook_proto_msgTypes = make([]protoimpl.MessageInfo, 9)
var file_tableau_protobuf_metabook_proto_goTypes = []interface{}{
	(*Metabook)(nil),               // 0: tableau.Metabook
	(*Metasheet)(nil),              // 1: tableau.Metasheet
	(*EnumDescriptor)(nil),         // 2: tableau.EnumDescriptor
	(*StructDescriptor)(nil),       // 3: tableau.StructDescriptor
	(*UnionDescriptor)(nil),        // 4: tableau.UnionDescriptor
	nil,                            // 5: tableau.Metabook.MetasheetMapEntry
	(*EnumDescriptor_Value)(nil),   // 6: tableau.EnumDescriptor.Value
	(*StructDescriptor_Field)(nil), // 7: tableau.StructDescriptor.Field
	(*UnionDescriptor_Value)(nil),  // 8: tableau.UnionDescriptor.Value
	(Mode)(0),                      // 9: tableau.Mode
	(Patch)(0),                     // 10: tableau.Patch
}
var file_tableau_protobuf_metabook_proto_depIdxs = []int32{
	5,  // 0: tableau.Metabook.metasheet_map:type_name -> tableau.Metabook.MetasheetMapEntry
	9,  // 1: tableau.Metasheet.mode:type_name -> tableau.Mode
	10, // 2: tableau.Metasheet.patch:type_name -> tableau.Patch
	6,  // 3: tableau.EnumDescriptor.values:type_name -> tableau.EnumDescriptor.Value
	7,  // 4: tableau.StructDescriptor.fields:type_name -> tableau.StructDescriptor.Field
	8,  // 5: tableau.UnionDescriptor.values:type_name -> tableau.UnionDescriptor.Value
	1,  // 6: tableau.Metabook.MetasheetMapEntry.value:type_name -> tableau.Metasheet
	7,  // [7:7] is the sub-list for method output_type
	7,  // [7:7] is the sub-list for method input_type
	7,  // [7:7] is the sub-list for extension type_name
	7,  // [7:7] is the sub-list for extension extendee
	0,  // [0:7] is the sub-list for field type_name
}

func init() { file_tableau_protobuf_metabook_proto_init() }
func file_tableau_protobuf_metabook_proto_init() {
	if File_tableau_protobuf_metabook_proto != nil {
		return
	}
	file_tableau_protobuf_tableau_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_tableau_protobuf_metabook_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Metabook); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_tableau_protobuf_metabook_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Metasheet); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_tableau_protobuf_metabook_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*EnumDescriptor); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_tableau_protobuf_metabook_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*StructDescriptor); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_tableau_protobuf_metabook_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*UnionDescriptor); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_tableau_protobuf_metabook_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*EnumDescriptor_Value); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_tableau_protobuf_metabook_proto_msgTypes[7].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*StructDescriptor_Field); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_tableau_protobuf_metabook_proto_msgTypes[8].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*UnionDescriptor_Value); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	file_tableau_protobuf_metabook_proto_msgTypes[6].OneofWrappers = []interface{}{}
	file_tableau_protobuf_metabook_proto_msgTypes[8].OneofWrappers = []interface{}{}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_tableau_protobuf_metabook_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   9,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_tableau_protobuf_metabook_proto_goTypes,
		DependencyIndexes: file_tableau_protobuf_metabook_proto_depIdxs,
		MessageInfos:      file_tableau_protobuf_metabook_proto_msgTypes,
	}.Build()
	File_tableau_protobuf_metabook_proto = out.File
	file_tableau_protobuf_metabook_proto_rawDesc = nil
	file_tableau_protobuf_metabook_proto_goTypes = nil
	file_tableau_protobuf_metabook_proto_depIdxs = nil
}
