package store

import (
	"bytes"
	"encoding/json"
	"strings"
	"time"

	"github.com/protocolbuffers/txtpbfmt/parser"
	"github.com/tableauio/tableau/store/jsonparser"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
)

type MarshalOptions struct {
	// Location represents the collection of time offsets in use in a geographical area.
	//  - If the name is "" or "UTC", LoadLocation returns UTC.
	//  - If the name is "Local", LoadLocation returns Local.
	//  - Otherwise, the name is taken to be a location name corresponding to a file in the
	//    IANA Time Zone database, such as "America/New_York", "Asia/Shanghai", and so on.
	//
	// See https://go.dev/src/time/zoneinfo_abbrs_windows.go.
	//
	// Default: "Local".
	LocationName string `yaml:"locationName"`
	// Output pretty format of JSON and Text, with multiline and indent.
	//
	// Default: false.
	Pretty bool

	// EmitUnpopulated specifies whether to emit unpopulated fields. It does not
	// emit unpopulated oneof fields or unpopulated extension fields.
	// The JSON value emitted for unpopulated fields are as follows:
	//  ╔═══════╤════════════════════════════╗
	//  ║ JSON  │ Protobuf field             ║
	//  ╠═══════╪════════════════════════════╣
	//  ║ false │ proto3 boolean fields      ║
	//  ║ 0     │ proto3 numeric fields      ║
	//  ║ ""    │ proto3 string/bytes fields ║
	//  ║ null  │ proto2 scalar fields       ║
	//  ║ null  │ message fields             ║
	//  ║ []    │ list fields                ║
	//  ║ {}    │ map fields                 ║
	//  ╚═══════╧════════════════════════════╝
	//
	// NOTE: worksheet with FieldPresence set as true ignore this option.
	//
	// Refer: https://github.com/protocolbuffers/protobuf/blob/main/docs/field_presence.md
	//
	// Default: false.
	EmitUnpopulated bool

	// EmitTimezones specifies whether to emit timestamp in string format with
	// timezones (as indicated by an offset).
	EmitTimezones bool

	// UseProtoNames uses proto field name instead of lowerCamelCase name in JSON
	// field names.
	UseProtoNames bool

	// UseEnumNumbers emits enum values as numbers.
	UseEnumNumbers bool
}

// MarshalToJSON marshals the given proto.Message in the JSON format.
// You can depend on the output being stable.
func MarshalToJSON(msg proto.Message, options *MarshalOptions) (out []byte, err error) {
	var location *time.Location
	if options.EmitTimezones {
		location, err = loadLocation(options.LocationName)
		if err != nil {
			return nil, err
		}
	}
	opts := protojson.MarshalOptions{
		EmitUnpopulated: options.EmitUnpopulated,
		UseProtoNames:   options.UseProtoNames,
		UseEnumNumbers:  options.UseEnumNumbers,
	}
	messageJSON, err := opts.Marshal(msg)
	if err != nil {
		return nil, err
	}
	// protojson always encodes Timestamp in UTC with a trailing Z. A match may
	// come from an ordinary string, but that only causes a harmless extra typed
	// traversal: rewriteJSONTimestamps changes Timestamp fields exclusively.
	rewriteTimestamps := options.EmitTimezones && bytes.Contains(messageJSON, []byte(`Z"`))
	if rewriteTimestamps {
		result, err := rewriteJSONTimestamps(msg, string(messageJSON), jsonparser.Fastjson, location, options.UseProtoNames)
		if err != nil {
			return nil, err
		}
		messageJSON = []byte(result)
	}
	if options.Pretty {
		prettyJSON := new(bytes.Buffer)
		prettyJSON.Grow(len(messageJSON) + len(messageJSON)/2)
		if err := json.Indent(prettyJSON, messageJSON, "", "    "); err != nil {
			return nil, err
		}
		return prettyJSON.Bytes(), nil
	}
	if rewriteTimestamps {
		return messageJSON, nil
	}
	compactJSON := new(bytes.Buffer)
	compactJSON.Grow(len(messageJSON))
	if err := json.Compact(compactJSON, messageJSON); err != nil {
		return nil, err
	}
	return compactJSON.Bytes(), nil
}

// MarshalToText marshals the given proto.Message in the text (textproto) format.
// You can depend on the output being stable.
func MarshalToText(msg proto.Message, pretty bool) (out []byte, err error) {
	if pretty {
		opts := prototext.MarshalOptions{
			Multiline: true,
			Indent:    "    ",
		}
		messageText, err := opts.Marshal(msg)
		if err != nil {
			return nil, err
		}
		// To obtain some degree of stability, the protobuf-go team recommend passing
		// the output of prototext through the [txtpbfmt](https://github.com/protocolbuffers/txtpbfmt)
		// program. The formatter can be directly invoked in Go using parser.Format.
		text, err := parser.Format(messageText)
		if err != nil {
			return nil, err
		}
		// remove last newline
		return bytes.TrimRight(text, "\n"), nil
	}

	messageText, err := prototext.Marshal(msg)
	if err != nil {
		return nil, err
	}
	// To obtain some degree of stability, remove redundant spaces/whitespace.
	// refer: https://stackoverflow.com/questions/37290693/how-to-remove-redundant-spaces-whitespace-from-a-string-in-golang
	text := strings.Join(strings.Fields(string(messageText)), " ")
	return []byte(text), nil
}

// MarshalToBin marshals the given proto.Message in the wire (binary) format.
// You can depend on the output being stable.
func MarshalToBin(msg proto.Message) (out []byte, err error) {
	// protobuf does not offer a canonical output today, so this format is not
	// guaranteed to match deterministic output from other protobuf libraries.
	// In addition, unknown fields may cause inconsistent output for otherwise
	// equal messages.
	// https://github.com/golang/protobuf/issues/1121
	options := proto.MarshalOptions{Deterministic: true}
	return options.Marshal(msg)
}
