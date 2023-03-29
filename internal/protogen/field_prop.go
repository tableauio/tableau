package protogen

import (
	"github.com/tableauio/tableau/proto/tableaupb"
	"google.golang.org/protobuf/proto"
)

var emptyFieldProp = &tableaupb.FieldProp{}

func IsEmptyFieldProp(prop *tableaupb.FieldProp) bool {
	return proto.Equal(emptyFieldProp, prop)
}

// ExtractMapFieldProp extracts the specified props which the map field recognizes.
func ExtractMapFieldProp(prop *tableaupb.FieldProp) *tableaupb.FieldProp {
	if prop == nil {
		return nil
	}
	p := &tableaupb.FieldProp{
		JsonName: prop.JsonName,
		Unique:   prop.Unique,
		Sequence: prop.Sequence,
		Fixed:    prop.Fixed,
		Size:     prop.Size,
	}
	if IsEmptyFieldProp(p) {
		return nil
	}
	return p
}

// ExtractListFieldProp extracts the specified props which the list field recognizes.
func ExtractListFieldProp(prop *tableaupb.FieldProp) *tableaupb.FieldProp {
	if prop == nil {
		return nil
	}
	p := &tableaupb.FieldProp{
		JsonName: prop.JsonName,
		Unique:   prop.Unique, // only for keyed list ?
		Sequence: prop.Sequence,
		Fixed:    prop.Fixed,
		Size:     prop.Size,
	}
	if IsEmptyFieldProp(p) {
		return nil
	}
	return p
}

// ExtractStructFieldProp extracts the specified props which the struct field recognizes.
func ExtractStructFieldProp(prop *tableaupb.FieldProp) *tableaupb.FieldProp {
	if prop == nil {
		return nil
	}
	p := &tableaupb.FieldProp{
		JsonName: prop.JsonName,
		Form:     prop.Form,
	}
	if IsEmptyFieldProp(p) {
		return nil
	}
	return p
}

// ExtractScalarFieldProp extracts the specified props which the scalar field recognizes.
func ExtractScalarFieldProp(prop *tableaupb.FieldProp) *tableaupb.FieldProp {
	if prop == nil {
		return nil
	}
	p := &tableaupb.FieldProp{
		JsonName: prop.JsonName,
		Range:    prop.Range,
		Refer:    prop.Refer,
		Default:  prop.Default,
	}
	if IsEmptyFieldProp(p) {
		return nil
	}
	return p
}
