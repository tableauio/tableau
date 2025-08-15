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
		Fixed:    prop.Fixed,
		Size:     prop.Size,
		Present:  prop.Present,
		Optional: prop.Optional,
		Patch:    prop.Patch,
		Sep:      prop.Sep,
		Subsep:   prop.Subsep,
		Number:   prop.Number,
	}
	if IsEmptyFieldProp(p) {
		return nil
	}
	return p
}

// ExtractListFieldProp extracts the specified props which the list field recognizes.
func ExtractListFieldProp(prop *tableaupb.FieldProp, isScalarList bool) *tableaupb.FieldProp {
	if prop == nil {
		return nil
	}
	p := &tableaupb.FieldProp{
		JsonName: prop.JsonName,
		Fixed:    prop.Fixed,
		Size:     prop.Size,
		Present:  prop.Present,
		Optional: prop.Optional,
		Patch:    prop.Patch,
		Form:     prop.Form, // for vertical incell union list
		Sep:      prop.Sep,
		Subsep:   prop.Subsep,
		Cross:    prop.Cross,
		Number:   prop.Number,
	}
	if isScalarList {
		p.Range = prop.Range
		p.Refer = prop.Refer
		p.Pattern = prop.Pattern
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
		Present:  prop.Present,
		Optional: prop.Optional,
		Patch:    prop.Patch,
		Sep:      prop.Sep,
		Subsep:   prop.Subsep,
		Number:   prop.Number,
	}
	if IsEmptyFieldProp(p) {
		return nil
	}
	return p
}

// ExtractScalarFieldProp extracts the specified props which the scalar field recognizes.
//
// FIXME(wenchy): wellknown type fields should also be supported.
func ExtractScalarFieldProp(prop *tableaupb.FieldProp) *tableaupb.FieldProp {
	if prop == nil {
		return nil
	}
	p := &tableaupb.FieldProp{
		JsonName: prop.JsonName,
		Unique:   prop.Unique,
		Sequence: prop.Sequence,
		Range:    prop.Range,
		Refer:    prop.Refer,
		Default:  prop.Default,
		Present:  prop.Present,
		Optional: prop.Optional,
		Patch:    prop.Patch,
		Pattern:  prop.Pattern,
		Number:   prop.Number,
	}
	if IsEmptyFieldProp(p) {
		return nil
	}
	return p
}
