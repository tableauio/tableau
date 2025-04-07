package store

import (
	"strconv"
	"strings"
	"time"

	"github.com/tableauio/tableau/internal/types"
	"github.com/tableauio/tableau/xerrors"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
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
	err = processTimeInJSON(msg.ProtoReflect(), &jsonStr, loc, "", useProtoNames)
	if err != nil {
		return "", xerrors.Wrap(err)
	}
	return jsonStr, nil
}

// References: https://github.com/protocolbuffers/protobuf-go/blob/v1.34.2/encoding/protojson/encode.go#L262
func fieldJSONName(fd protoreflect.FieldDescriptor, useProtoNames bool) string {
	if useProtoNames {
		return fd.TextName()
	}
	return fd.JSONName()
}

func processTimeInJSON(msg protoreflect.Message, jsonStr *string, loc *time.Location, prefix string, useProtoNames bool) error {
	prefix = strings.TrimPrefix(prefix, ".")
	if msg.Descriptor().FullName() == types.WellKnownMessageTimestamp {
		replaced, err := sjson.Set(*jsonStr, prefix, formatTimestamp(gjson.Get(*jsonStr, prefix).String(), loc))
		if err != nil {
			return err
		}
		*jsonStr = replaced
	} else {
		msg.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
			if fd.Kind() != protoreflect.MessageKind {
				return true
			}
			if fd.IsMap() {
				if fd.MapValue().Kind() != protoreflect.MessageKind {
					return true
				}
				v.Map().Range(func(key protoreflect.MapKey, value protoreflect.Value) bool {
					processTimeInJSON(value.Message(), jsonStr, loc, prefix+"."+fieldJSONName(fd, useProtoNames)+"."+key.String(), useProtoNames)
					return true
				})
			} else if fd.IsList() {
				for i := 0; i < v.List().Len(); i++ {
					processTimeInJSON(v.List().Get(i).Message(), jsonStr, loc, prefix+"."+fieldJSONName(fd, useProtoNames)+"."+strconv.Itoa(i), useProtoNames)
				}
			} else {
				processTimeInJSON(v.Message(), jsonStr, loc, prefix+"."+fieldJSONName(fd, useProtoNames), useProtoNames)
			}
			return true
		})
	}
	return nil
}
