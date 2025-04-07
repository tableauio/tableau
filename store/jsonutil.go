package store

import (
	"time"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/tableauio/tableau/internal/types"
	"github.com/tableauio/tableau/xerrors"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Format a timestamp to the desired string format
func formatTimestamp(ts string, loc *time.Location) string {
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return ts // Return the original string if parsing fails
	}
	localTime := t.In(loc)
	return localTime.Format(time.RFC3339Nano)
}

// processWhenEmitTimezones emits timestamp in string format with timezones
// (as indicated by an offset).
//
// # Problem
//
// A proto3 JSON serializer should always use UTC (as indicated by "Z") when
// printing the Timestamp type and a proto3 JSON parser should be able to
// accept both UTC and other timezones (as indicated by an offset).
//
// For example, "2017-01-15T01:30:15.01Z" encodes 15.01 seconds past 01:30 UTC
// on January 15, 2017.
//
// # Solution
//
// This function processes the JSON string and replaces all timestamps with
// the desired format.
//
// # References
//
//   - https://pkg.go.dev/google.golang.org/protobuf/types/known/timestamppb#hdr-JSON_Mapping-Timestamp
//   - https://protobuf.dev/reference/protobuf/google.protobuf/#timestamp
//   - RFC 3339: https://tools.ietf.org/html/rfc3339
func processWhenEmitTimezones(msg proto.Message, jsonStr string, locationName string, useProtoNames bool) (string, error) {
	loc, err := time.LoadLocation(locationName)
	if err != nil {
		return "", xerrors.Wrap(err)
	}
	root, err := sonic.Get([]byte(jsonStr))
	if err != nil {
		return "", xerrors.Wrap(err)
	}
	_, err = processTimeInJSON(msg.ProtoReflect(), &root, loc, useProtoNames)
	if err != nil {
		return "", xerrors.Wrap(err)
	}
	return root.Raw()
}

// References: https://github.com/protocolbuffers/protobuf-go/blob/v1.34.2/encoding/protojson/encode.go#L262
func fieldJSONName(fd protoreflect.FieldDescriptor, useProtoNames bool) string {
	if useProtoNames {
		return fd.TextName()
	}
	return fd.JSONName()
}

func processTimeInJSON(msg protoreflect.Message, node *ast.Node, loc *time.Location, useProtoNames bool) (*ast.Node, error) {
	if msg.Descriptor().FullName() == types.WellKnownMessageTimestamp {
		raw, err := node.StrictString()
		if err != nil {
			return nil, err
		}
		newNode := ast.NewString(formatTimestamp(raw, loc))
		return &newNode, nil
	} else {
		var e error
		msg.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
			if fd.Kind() != protoreflect.MessageKind {
				return true
			}
			if fd.IsMap() {
				if fd.MapValue().Kind() != protoreflect.MessageKind {
					return true
				}
				subNode := node.Get(fieldJSONName(fd, useProtoNames))
				v.Map().Range(func(key protoreflect.MapKey, value protoreflect.Value) bool {
					newNode, err := processTimeInJSON(value.Message(), subNode.Get(key.String()), loc, useProtoNames)
					if err != nil {
						e = err
						return false
					}
					if newNode != nil {
						subNode.Set(key.String(), *newNode)
					}
					return true
				})
			} else if fd.IsList() {
				subNode := node.Get(fieldJSONName(fd, useProtoNames))
				for i := 0; i < v.List().Len(); i++ {
					newNode, err := processTimeInJSON(v.List().Get(i).Message(), subNode.Index(i), loc, useProtoNames)
					if err != nil {
						e = err
						break
					}
					if newNode != nil {
						subNode.SetByIndex(i, *newNode)
					}
				}
			} else {
				newNode, err := processTimeInJSON(v.Message(), node.Get(fieldJSONName(fd, useProtoNames)), loc, useProtoNames)
				if err != nil {
					e = err
				}
				if newNode != nil {
					node.Set(fieldJSONName(fd, useProtoNames), *newNode)
				}
			}
			return e == nil
		})
		return nil, e
	}
}
