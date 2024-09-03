// Protoconf - Tableau's data interchange format
// https://tableauio.github.io/

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        v3.21.12
// source: tableau/protobuf/workbook.proto

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

type Workbook struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name       string           `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"` // book name without suffix
	Options    *WorkbookOptions `protobuf:"bytes,2,opt,name=options,proto3" json:"options,omitempty"`
	Worksheets []*Worksheet     `protobuf:"bytes,3,rep,name=worksheets,proto3" json:"worksheets,omitempty"`
	Imports    map[string]int32 `protobuf:"bytes,4,rep,name=imports,proto3" json:"imports,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"varint,2,opt,name=value,proto3"` // imported proto files
}

func (x *Workbook) Reset() {
	*x = Workbook{}
	if protoimpl.UnsafeEnabled {
		mi := &file_tableau_protobuf_workbook_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Workbook) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Workbook) ProtoMessage() {}

func (x *Workbook) ProtoReflect() protoreflect.Message {
	mi := &file_tableau_protobuf_workbook_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Workbook.ProtoReflect.Descriptor instead.
func (*Workbook) Descriptor() ([]byte, []int) {
	return file_tableau_protobuf_workbook_proto_rawDescGZIP(), []int{0}
}

func (x *Workbook) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Workbook) GetOptions() *WorkbookOptions {
	if x != nil {
		return x.Options
	}
	return nil
}

func (x *Workbook) GetWorksheets() []*Worksheet {
	if x != nil {
		return x.Worksheets
	}
	return nil
}

func (x *Workbook) GetImports() map[string]int32 {
	if x != nil {
		return x.Imports
	}
	return nil
}

type Worksheet struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name    string            `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Options *WorksheetOptions `protobuf:"bytes,2,opt,name=options,proto3" json:"options,omitempty"`
	Fields  []*Field          `protobuf:"bytes,3,rep,name=fields,proto3" json:"fields,omitempty"`
}

func (x *Worksheet) Reset() {
	*x = Worksheet{}
	if protoimpl.UnsafeEnabled {
		mi := &file_tableau_protobuf_workbook_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Worksheet) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Worksheet) ProtoMessage() {}

func (x *Worksheet) ProtoReflect() protoreflect.Message {
	mi := &file_tableau_protobuf_workbook_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Worksheet.ProtoReflect.Descriptor instead.
func (*Worksheet) Descriptor() ([]byte, []int) {
	return file_tableau_protobuf_workbook_proto_rawDescGZIP(), []int{1}
}

func (x *Worksheet) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Worksheet) GetOptions() *WorksheetOptions {
	if x != nil {
		return x.Options
	}
	return nil
}

func (x *Worksheet) GetFields() []*Field {
	if x != nil {
		return x.Fields
	}
	return nil
}

type Field struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Field tag number
	// Note: only for enum/struct/union type definition in sheet
	Number    int32            `protobuf:"varint,1,opt,name=number,proto3" json:"number,omitempty"`
	Name      string           `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
	Alias     string           `protobuf:"bytes,3,opt,name=alias,proto3" json:"alias,omitempty"`
	Type      string           `protobuf:"bytes,4,opt,name=type,proto3" json:"type,omitempty"`
	FullType  string           `protobuf:"bytes,5,opt,name=full_type,json=fullType,proto3" json:"full_type,omitempty"`
	ListEntry *Field_ListEntry `protobuf:"bytes,6,opt,name=list_entry,json=listEntry,proto3" json:"list_entry,omitempty"`
	MapEntry  *Field_MapEntry  `protobuf:"bytes,7,opt,name=map_entry,json=mapEntry,proto3" json:"map_entry,omitempty"`
	// Indicate this field's related type is predefined.
	// - enum: enum type
	// - struct: message type
	// - list: list's element type
	// - map: map's value type
	Predefined bool          `protobuf:"varint,8,opt,name=predefined,proto3" json:"predefined,omitempty"`
	Options    *FieldOptions `protobuf:"bytes,9,opt,name=options,proto3" json:"options,omitempty"`
	// This field can be struct, list or map if sub fields's length is not 0.
	Fields []*Field `protobuf:"bytes,10,rep,name=fields,proto3" json:"fields,omitempty"`
}

func (x *Field) Reset() {
	*x = Field{}
	if protoimpl.UnsafeEnabled {
		mi := &file_tableau_protobuf_workbook_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Field) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Field) ProtoMessage() {}

func (x *Field) ProtoReflect() protoreflect.Message {
	mi := &file_tableau_protobuf_workbook_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Field.ProtoReflect.Descriptor instead.
func (*Field) Descriptor() ([]byte, []int) {
	return file_tableau_protobuf_workbook_proto_rawDescGZIP(), []int{2}
}

func (x *Field) GetNumber() int32 {
	if x != nil {
		return x.Number
	}
	return 0
}

func (x *Field) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Field) GetAlias() string {
	if x != nil {
		return x.Alias
	}
	return ""
}

func (x *Field) GetType() string {
	if x != nil {
		return x.Type
	}
	return ""
}

func (x *Field) GetFullType() string {
	if x != nil {
		return x.FullType
	}
	return ""
}

func (x *Field) GetListEntry() *Field_ListEntry {
	if x != nil {
		return x.ListEntry
	}
	return nil
}

func (x *Field) GetMapEntry() *Field_MapEntry {
	if x != nil {
		return x.MapEntry
	}
	return nil
}

func (x *Field) GetPredefined() bool {
	if x != nil {
		return x.Predefined
	}
	return false
}

func (x *Field) GetOptions() *FieldOptions {
	if x != nil {
		return x.Options
	}
	return nil
}

func (x *Field) GetFields() []*Field {
	if x != nil {
		return x.Fields
	}
	return nil
}

type Field_ListEntry struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ElemType     string `protobuf:"bytes,1,opt,name=elem_type,json=elemType,proto3" json:"elem_type,omitempty"`
	ElemFullType string `protobuf:"bytes,2,opt,name=elem_full_type,json=elemFullType,proto3" json:"elem_full_type,omitempty"`
}

func (x *Field_ListEntry) Reset() {
	*x = Field_ListEntry{}
	if protoimpl.UnsafeEnabled {
		mi := &file_tableau_protobuf_workbook_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Field_ListEntry) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Field_ListEntry) ProtoMessage() {}

func (x *Field_ListEntry) ProtoReflect() protoreflect.Message {
	mi := &file_tableau_protobuf_workbook_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Field_ListEntry.ProtoReflect.Descriptor instead.
func (*Field_ListEntry) Descriptor() ([]byte, []int) {
	return file_tableau_protobuf_workbook_proto_rawDescGZIP(), []int{2, 0}
}

func (x *Field_ListEntry) GetElemType() string {
	if x != nil {
		return x.ElemType
	}
	return ""
}

func (x *Field_ListEntry) GetElemFullType() string {
	if x != nil {
		return x.ElemFullType
	}
	return ""
}

type Field_MapEntry struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	KeyType       string `protobuf:"bytes,1,opt,name=key_type,json=keyType,proto3" json:"key_type,omitempty"`
	ValueType     string `protobuf:"bytes,2,opt,name=value_type,json=valueType,proto3" json:"value_type,omitempty"`
	ValueFullType string `protobuf:"bytes,3,opt,name=value_full_type,json=valueFullType,proto3" json:"value_full_type,omitempty"`
}

func (x *Field_MapEntry) Reset() {
	*x = Field_MapEntry{}
	if protoimpl.UnsafeEnabled {
		mi := &file_tableau_protobuf_workbook_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Field_MapEntry) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Field_MapEntry) ProtoMessage() {}

func (x *Field_MapEntry) ProtoReflect() protoreflect.Message {
	mi := &file_tableau_protobuf_workbook_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Field_MapEntry.ProtoReflect.Descriptor instead.
func (*Field_MapEntry) Descriptor() ([]byte, []int) {
	return file_tableau_protobuf_workbook_proto_rawDescGZIP(), []int{2, 1}
}

func (x *Field_MapEntry) GetKeyType() string {
	if x != nil {
		return x.KeyType
	}
	return ""
}

func (x *Field_MapEntry) GetValueType() string {
	if x != nil {
		return x.ValueType
	}
	return ""
}

func (x *Field_MapEntry) GetValueFullType() string {
	if x != nil {
		return x.ValueFullType
	}
	return ""
}

var File_tableau_protobuf_workbook_proto protoreflect.FileDescriptor

var file_tableau_protobuf_workbook_proto_rawDesc = []byte{
	0x0a, 0x1f, 0x74, 0x61, 0x62, 0x6c, 0x65, 0x61, 0x75, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62,
	0x75, 0x66, 0x2f, 0x77, 0x6f, 0x72, 0x6b, 0x62, 0x6f, 0x6f, 0x6b, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x12, 0x07, 0x74, 0x61, 0x62, 0x6c, 0x65, 0x61, 0x75, 0x1a, 0x1e, 0x74, 0x61, 0x62, 0x6c,
	0x65, 0x61, 0x75, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x74, 0x61, 0x62,
	0x6c, 0x65, 0x61, 0x75, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0xfc, 0x01, 0x0a, 0x08, 0x57,
	0x6f, 0x72, 0x6b, 0x62, 0x6f, 0x6f, 0x6b, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x32, 0x0a, 0x07, 0x6f,
	0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x18, 0x2e, 0x74,
	0x61, 0x62, 0x6c, 0x65, 0x61, 0x75, 0x2e, 0x57, 0x6f, 0x72, 0x6b, 0x62, 0x6f, 0x6f, 0x6b, 0x4f,
	0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x52, 0x07, 0x6f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x12,
	0x32, 0x0a, 0x0a, 0x77, 0x6f, 0x72, 0x6b, 0x73, 0x68, 0x65, 0x65, 0x74, 0x73, 0x18, 0x03, 0x20,
	0x03, 0x28, 0x0b, 0x32, 0x12, 0x2e, 0x74, 0x61, 0x62, 0x6c, 0x65, 0x61, 0x75, 0x2e, 0x57, 0x6f,
	0x72, 0x6b, 0x73, 0x68, 0x65, 0x65, 0x74, 0x52, 0x0a, 0x77, 0x6f, 0x72, 0x6b, 0x73, 0x68, 0x65,
	0x65, 0x74, 0x73, 0x12, 0x38, 0x0a, 0x07, 0x69, 0x6d, 0x70, 0x6f, 0x72, 0x74, 0x73, 0x18, 0x04,
	0x20, 0x03, 0x28, 0x0b, 0x32, 0x1e, 0x2e, 0x74, 0x61, 0x62, 0x6c, 0x65, 0x61, 0x75, 0x2e, 0x57,
	0x6f, 0x72, 0x6b, 0x62, 0x6f, 0x6f, 0x6b, 0x2e, 0x49, 0x6d, 0x70, 0x6f, 0x72, 0x74, 0x73, 0x45,
	0x6e, 0x74, 0x72, 0x79, 0x52, 0x07, 0x69, 0x6d, 0x70, 0x6f, 0x72, 0x74, 0x73, 0x1a, 0x3a, 0x0a,
	0x0c, 0x49, 0x6d, 0x70, 0x6f, 0x72, 0x74, 0x73, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x12, 0x10, 0x0a,
	0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12,
	0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x05, 0x52, 0x05,
	0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38, 0x01, 0x22, 0x7c, 0x0a, 0x09, 0x57, 0x6f, 0x72,
	0x6b, 0x73, 0x68, 0x65, 0x65, 0x74, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x33, 0x0a, 0x07, 0x6f, 0x70,
	0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x19, 0x2e, 0x74, 0x61,
	0x62, 0x6c, 0x65, 0x61, 0x75, 0x2e, 0x57, 0x6f, 0x72, 0x6b, 0x73, 0x68, 0x65, 0x65, 0x74, 0x4f,
	0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x52, 0x07, 0x6f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x12,
	0x26, 0x0a, 0x06, 0x66, 0x69, 0x65, 0x6c, 0x64, 0x73, 0x18, 0x03, 0x20, 0x03, 0x28, 0x0b, 0x32,
	0x0e, 0x2e, 0x74, 0x61, 0x62, 0x6c, 0x65, 0x61, 0x75, 0x2e, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x52,
	0x06, 0x66, 0x69, 0x65, 0x6c, 0x64, 0x73, 0x22, 0xa0, 0x04, 0x0a, 0x05, 0x46, 0x69, 0x65, 0x6c,
	0x64, 0x12, 0x16, 0x0a, 0x06, 0x6e, 0x75, 0x6d, 0x62, 0x65, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x05, 0x52, 0x06, 0x6e, 0x75, 0x6d, 0x62, 0x65, 0x72, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d,
	0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x14, 0x0a,
	0x05, 0x61, 0x6c, 0x69, 0x61, 0x73, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x61, 0x6c,
	0x69, 0x61, 0x73, 0x12, 0x12, 0x0a, 0x04, 0x74, 0x79, 0x70, 0x65, 0x18, 0x04, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65, 0x12, 0x1b, 0x0a, 0x09, 0x66, 0x75, 0x6c, 0x6c, 0x5f,
	0x74, 0x79, 0x70, 0x65, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x66, 0x75, 0x6c, 0x6c,
	0x54, 0x79, 0x70, 0x65, 0x12, 0x37, 0x0a, 0x0a, 0x6c, 0x69, 0x73, 0x74, 0x5f, 0x65, 0x6e, 0x74,
	0x72, 0x79, 0x18, 0x06, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x18, 0x2e, 0x74, 0x61, 0x62, 0x6c, 0x65,
	0x61, 0x75, 0x2e, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x2e, 0x4c, 0x69, 0x73, 0x74, 0x45, 0x6e, 0x74,
	0x72, 0x79, 0x52, 0x09, 0x6c, 0x69, 0x73, 0x74, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x12, 0x34, 0x0a,
	0x09, 0x6d, 0x61, 0x70, 0x5f, 0x65, 0x6e, 0x74, 0x72, 0x79, 0x18, 0x07, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x17, 0x2e, 0x74, 0x61, 0x62, 0x6c, 0x65, 0x61, 0x75, 0x2e, 0x46, 0x69, 0x65, 0x6c, 0x64,
	0x2e, 0x4d, 0x61, 0x70, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x52, 0x08, 0x6d, 0x61, 0x70, 0x45, 0x6e,
	0x74, 0x72, 0x79, 0x12, 0x1e, 0x0a, 0x0a, 0x70, 0x72, 0x65, 0x64, 0x65, 0x66, 0x69, 0x6e, 0x65,
	0x64, 0x18, 0x08, 0x20, 0x01, 0x28, 0x08, 0x52, 0x0a, 0x70, 0x72, 0x65, 0x64, 0x65, 0x66, 0x69,
	0x6e, 0x65, 0x64, 0x12, 0x2f, 0x0a, 0x07, 0x6f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0x09,
	0x20, 0x01, 0x28, 0x0b, 0x32, 0x15, 0x2e, 0x74, 0x61, 0x62, 0x6c, 0x65, 0x61, 0x75, 0x2e, 0x46,
	0x69, 0x65, 0x6c, 0x64, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x52, 0x07, 0x6f, 0x70, 0x74,
	0x69, 0x6f, 0x6e, 0x73, 0x12, 0x26, 0x0a, 0x06, 0x66, 0x69, 0x65, 0x6c, 0x64, 0x73, 0x18, 0x0a,
	0x20, 0x03, 0x28, 0x0b, 0x32, 0x0e, 0x2e, 0x74, 0x61, 0x62, 0x6c, 0x65, 0x61, 0x75, 0x2e, 0x46,
	0x69, 0x65, 0x6c, 0x64, 0x52, 0x06, 0x66, 0x69, 0x65, 0x6c, 0x64, 0x73, 0x1a, 0x4e, 0x0a, 0x09,
	0x4c, 0x69, 0x73, 0x74, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x12, 0x1b, 0x0a, 0x09, 0x65, 0x6c, 0x65,
	0x6d, 0x5f, 0x74, 0x79, 0x70, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x65, 0x6c,
	0x65, 0x6d, 0x54, 0x79, 0x70, 0x65, 0x12, 0x24, 0x0a, 0x0e, 0x65, 0x6c, 0x65, 0x6d, 0x5f, 0x66,
	0x75, 0x6c, 0x6c, 0x5f, 0x74, 0x79, 0x70, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0c,
	0x65, 0x6c, 0x65, 0x6d, 0x46, 0x75, 0x6c, 0x6c, 0x54, 0x79, 0x70, 0x65, 0x1a, 0x6c, 0x0a, 0x08,
	0x4d, 0x61, 0x70, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x12, 0x19, 0x0a, 0x08, 0x6b, 0x65, 0x79, 0x5f,
	0x74, 0x79, 0x70, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x6b, 0x65, 0x79, 0x54,
	0x79, 0x70, 0x65, 0x12, 0x1d, 0x0a, 0x0a, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x5f, 0x74, 0x79, 0x70,
	0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x54, 0x79,
	0x70, 0x65, 0x12, 0x26, 0x0a, 0x0f, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x5f, 0x66, 0x75, 0x6c, 0x6c,
	0x5f, 0x74, 0x79, 0x70, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0d, 0x76, 0x61, 0x6c,
	0x75, 0x65, 0x46, 0x75, 0x6c, 0x6c, 0x54, 0x79, 0x70, 0x65, 0x42, 0x2e, 0x5a, 0x2c, 0x67, 0x69,
	0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x74, 0x61, 0x62, 0x6c, 0x65, 0x61, 0x75,
	0x69, 0x6f, 0x2f, 0x74, 0x61, 0x62, 0x6c, 0x65, 0x61, 0x75, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x2f, 0x74, 0x61, 0x62, 0x6c, 0x65, 0x61, 0x75, 0x70, 0x62, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x33,
}

var (
	file_tableau_protobuf_workbook_proto_rawDescOnce sync.Once
	file_tableau_protobuf_workbook_proto_rawDescData = file_tableau_protobuf_workbook_proto_rawDesc
)

func file_tableau_protobuf_workbook_proto_rawDescGZIP() []byte {
	file_tableau_protobuf_workbook_proto_rawDescOnce.Do(func() {
		file_tableau_protobuf_workbook_proto_rawDescData = protoimpl.X.CompressGZIP(file_tableau_protobuf_workbook_proto_rawDescData)
	})
	return file_tableau_protobuf_workbook_proto_rawDescData
}

var file_tableau_protobuf_workbook_proto_msgTypes = make([]protoimpl.MessageInfo, 6)
var file_tableau_protobuf_workbook_proto_goTypes = []interface{}{
	(*Workbook)(nil),         // 0: tableau.Workbook
	(*Worksheet)(nil),        // 1: tableau.Worksheet
	(*Field)(nil),            // 2: tableau.Field
	nil,                      // 3: tableau.Workbook.ImportsEntry
	(*Field_ListEntry)(nil),  // 4: tableau.Field.ListEntry
	(*Field_MapEntry)(nil),   // 5: tableau.Field.MapEntry
	(*WorkbookOptions)(nil),  // 6: tableau.WorkbookOptions
	(*WorksheetOptions)(nil), // 7: tableau.WorksheetOptions
	(*FieldOptions)(nil),     // 8: tableau.FieldOptions
}
var file_tableau_protobuf_workbook_proto_depIdxs = []int32{
	6, // 0: tableau.Workbook.options:type_name -> tableau.WorkbookOptions
	1, // 1: tableau.Workbook.worksheets:type_name -> tableau.Worksheet
	3, // 2: tableau.Workbook.imports:type_name -> tableau.Workbook.ImportsEntry
	7, // 3: tableau.Worksheet.options:type_name -> tableau.WorksheetOptions
	2, // 4: tableau.Worksheet.fields:type_name -> tableau.Field
	4, // 5: tableau.Field.list_entry:type_name -> tableau.Field.ListEntry
	5, // 6: tableau.Field.map_entry:type_name -> tableau.Field.MapEntry
	8, // 7: tableau.Field.options:type_name -> tableau.FieldOptions
	2, // 8: tableau.Field.fields:type_name -> tableau.Field
	9, // [9:9] is the sub-list for method output_type
	9, // [9:9] is the sub-list for method input_type
	9, // [9:9] is the sub-list for extension type_name
	9, // [9:9] is the sub-list for extension extendee
	0, // [0:9] is the sub-list for field type_name
}

func init() { file_tableau_protobuf_workbook_proto_init() }
func file_tableau_protobuf_workbook_proto_init() {
	if File_tableau_protobuf_workbook_proto != nil {
		return
	}
	file_tableau_protobuf_tableau_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_tableau_protobuf_workbook_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Workbook); i {
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
		file_tableau_protobuf_workbook_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Worksheet); i {
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
		file_tableau_protobuf_workbook_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Field); i {
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
		file_tableau_protobuf_workbook_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Field_ListEntry); i {
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
		file_tableau_protobuf_workbook_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Field_MapEntry); i {
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
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_tableau_protobuf_workbook_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   6,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_tableau_protobuf_workbook_proto_goTypes,
		DependencyIndexes: file_tableau_protobuf_workbook_proto_depIdxs,
		MessageInfos:      file_tableau_protobuf_workbook_proto_msgTypes,
	}.Build()
	File_tableau_protobuf_workbook_proto = out.File
	file_tableau_protobuf_workbook_proto_rawDesc = nil
	file_tableau_protobuf_workbook_proto_goTypes = nil
	file_tableau_protobuf_workbook_proto_depIdxs = nil
}
