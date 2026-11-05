package protogen

import (
	"github.com/tableauio/tableau/internal/types"
	"github.com/tableauio/tableau/internal/x/xerrors"
	"github.com/tableauio/tableau/proto/tableaupb"
	"google.golang.org/protobuf/proto"
)

// checkVpropAllowed rejects ValueProp unless the map is an incell scalar
// (or enum-value) map. Vertical/horizontal and message-valued maps do not
// consume vprop, so a non-empty second prop group is a user error.
func checkVpropAllowed(desc *types.MapDescriptor, incellScalar bool) error {
	if desc.ValueProp.Text != "" && !incellScalar {
		return xerrors.Newf("vprop is only valid for incell scalar maps, got: %s", desc.ValueProp.Text)
	}
	return nil
}

// typeWithValueProp appends the raw second prop group to a value type so
// parseBasicField can sink vprop onto a generated Key/Value struct field.
func typeWithValueProp(valueType string, valueProp types.PropDescriptor) string {
	if valueProp.Text == "" {
		return valueType
	}
	return valueType + valueProp.RawProp()
}

var emptyFieldProp = &tableaupb.FieldProp{}

func IsEmptyFieldProp(prop *tableaupb.FieldProp) bool {
	return proto.Equal(emptyFieldProp, prop)
}

// ExtractMapFieldProp extracts the specified props which the map field recognizes.
func ExtractMapFieldProp(prop *tableaupb.FieldProp, layout tableaupb.Layout) *tableaupb.FieldProp {
	if prop == nil {
		return nil
	}
	p := &tableaupb.FieldProp{
		JsonName:        prop.JsonName,
		Fixed:           prop.Fixed,
		Size:            prop.Size,
		Present:         prop.Present,
		Optional:        prop.Optional,
		Patch:           prop.Patch,
		Sep:             prop.Sep,
		Subsep:          prop.Subsep,
		ValidateComplex: prop.ValidateComplex,
		ValidateMessage: prop.ValidateMessage,
	}
	switch layout {
	case tableaupb.Layout_LAYOUT_HORIZONTAL, tableaupb.Layout_LAYOUT_INCELL:
		p.Aggregate = prop.Aggregate
	}
	if IsEmptyFieldProp(p) {
		return nil
	}
	return p
}

// ExtractListFieldProp extracts the specified props which the list field recognizes.
func ExtractListFieldProp(prop *tableaupb.FieldProp, isScalarList bool, layout tableaupb.Layout) *tableaupb.FieldProp {
	if prop == nil {
		return nil
	}
	p := &tableaupb.FieldProp{
		JsonName:        prop.JsonName,
		Present:         prop.Present,
		Optional:        prop.Optional,
		Patch:           prop.Patch,
		Form:            prop.Form, // for vertical incell union list
		Sep:             prop.Sep,
		Subsep:          prop.Subsep,
		ValidateComplex: prop.ValidateComplex,
		ValidateMessage: prop.ValidateMessage,
	}
	if isScalarList {
		p.Range = prop.Range
		p.Refer = prop.Refer
		p.Pattern = prop.Pattern
		p.Validate = prop.Validate
	}
	switch layout {
	case tableaupb.Layout_LAYOUT_HORIZONTAL:
		p.Fixed = prop.Fixed
		p.Size = prop.Size
		p.Aggregate = prop.Aggregate
	case tableaupb.Layout_LAYOUT_INCELL:
		p.Aggregate = prop.Aggregate
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
		JsonName:        prop.JsonName,
		Form:            prop.Form,
		Present:         prop.Present,
		Optional:        prop.Optional,
		Patch:           prop.Patch,
		Sep:             prop.Sep,
		Subsep:          prop.Subsep,
		ValidateMessage: prop.ValidateMessage,
	}
	if IsEmptyFieldProp(p) {
		return nil
	}
	return p
}

// ExtractScalarFieldProp extracts the specified props which the scalar field recognizes.
//
// FIXME(wenchy): wellknown type fields should also be supported. In fact, it's
// supported now, but it's not well tested and documented.
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
		Order:    prop.Order,
		Validate: prop.Validate,
	}
	if IsEmptyFieldProp(p) {
		return nil
	}
	return p
}
