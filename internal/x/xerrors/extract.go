package xerrors

import (
	"errors"
	"maps"
)

// errorEntry is a temporary construction record. Once scope inheritance is
// resolved, Inspect converts it to the canonical typed ErrorDetail.
type errorEntry struct {
	cause           error
	fields          map[string]any
	preserveMessage bool // typed details already contain their intended message
}

type multiUnwrapper interface{ Unwrap() []error }
type fieldLayer struct {
	fields map[string]any
	shared bool
}

func extractEntries(err error) []errorEntry {
	if err == nil {
		return nil
	}
	var layers []fieldLayer
	for cur := err; cur != nil; cur = errors.Unwrap(cur) {
		if e, ok := cur.(*Error); ok {
			if e == nil {
				return nil
			}
			var entries []errorEntry
			for _, detail := range e.Details {
				if detail != nil {
					entries = append(entries, errorEntry{cause: detail.cause, fields: detail.fields(), preserveMessage: true})
				}
			}
			return inheritFields(entries, layers)
		}
		if _, ok := cur.(*collected); ok {
			// Wrappers outside a collector snapshot describe the snapshot, rather
			// than all failures already stored in the collector tree.
			for i := range layers {
				layers[i].shared = false
			}
		}
		if fc, ok := cur.(fieldsCarrier); ok {
			layers = append(layers, fieldLayer{fields: fc.Fields(), shared: true})
		}
		if mu, ok := cur.(multiUnwrapper); ok {
			var entries []errorEntry
			for _, child := range mu.Unwrap() {
				entries = append(entries, extractEntries(child)...)
			}
			return inheritFields(entries, layers)
		}
	}
	fields := make(map[string]any)
	for _, layer := range layers {
		mergeFields(fields, layer.fields)
	}
	return []errorEntry{{cause: err, fields: fields}}
}

// Inner values win; fields outside a collector snapshot stay local unless
// the snapshot contains exactly one error.
func inheritFields(entries []errorEntry, layers []fieldLayer) []errorEntry {
	for i := len(layers) - 1; i >= 0; i-- {
		layer := layers[i]
		if len(layer.fields) == 0 || (!layer.shared && len(entries) != 1) {
			continue
		}
		for j := range entries {
			fields := maps.Clone(layer.fields)
			mergeFields(fields, entries[j].fields)
			entries[j].fields = fields
		}
	}
	return entries
}

// An inner code must not inherit a description belonging to an outer code.
func mergeFields(dst, src map[string]any) {
	if dst[KeyErrCode] != nil && src[KeyErrCode] != nil && errorField(dst, KeyErrCode) != errorField(src, KeyErrCode) {
		delete(dst, KeyErrDesc)
	}
	maps.Copy(dst, src)
}
