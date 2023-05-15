package mexporter

import (
	"bytes"
	"encoding/json"

	"github.com/protocolbuffers/txtpbfmt/parser"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
)

type MarshalOptions struct {
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

	// UseProtoNames uses proto field name instead of lowerCamelCase name in JSON
	// field names.
	UseProtoNames bool

	// UseEnumNumbers emits enum values as numbers.
	UseEnumNumbers bool
}

// marshalToJSON marshals the given proto.Message in the JSON format.
// You can depend on the output being stable.
func marshalToJSON(msg proto.Message, options *MarshalOptions) (out []byte, err error) {
	opts := protojson.MarshalOptions{
		EmitUnpopulated: options.EmitUnpopulated,
		UseProtoNames:   options.UseProtoNames,
		UseEnumNumbers:  options.UseEnumNumbers,
	}
	messageJSON, err := opts.Marshal(msg)
	if err != nil {
		return nil, err
	}
	// protojson does not offer a "deterministic" field ordering, but fields
	// are still ordered consistently by their index. However, protojson can
	// output inconsistent whitespace for some reason, therefore it is
	// suggested to use a formatter to ensure consistent formatting.
	// https://github.com/golang/protobuf/issues/1373
	stableJSON := new(bytes.Buffer)
	if err = json.Compact(stableJSON, messageJSON); err != nil {
		return nil, err
	}
	if options.Pretty {
		prettyJSON := new(bytes.Buffer)
		if err := json.Indent(prettyJSON, stableJSON.Bytes(), "", "    "); err != nil {
			return nil, err
		}
		return prettyJSON.Bytes(), nil
	}
	return stableJSON.Bytes(), nil
}

// marshalToText marshals the given proto.Message in the text (textproto) format.
// You can depend on the output being stable.
func marshalToText(msg proto.Message, pretty bool) (out []byte, err error) {
	messageText, err := func() ([]byte, error) {
		if pretty {
			opts := prototext.MarshalOptions{
				Multiline: true,
				Indent:    "    ",
			}
			return opts.Marshal(msg)
		}
		return prototext.Marshal(msg)
	}()
	if err != nil {
		return nil, err
	}
	// To obtain some degree of stability, the protobuf-go team recommend passing
	// the output of prototext through the [txtpbfmt](https://github.com/protocolbuffers/txtpbfmt)
	// program. The formatter can be directly invoked in Go using parser.Format.
	return parser.Format(messageText)
}

// marshalToBin marshals the given proto.Message in the wire (binary) format.
// You can depend on the output being stable.
func marshalToBin(msg proto.Message) (out []byte, err error) {
	// protobuf does not offer a canonical output today, so this format is not
	// guaranteed to match deterministic output from other protobuf libraries.
	// In addition, unknown fields may cause inconsistent output for otherwise
	// equal messages.
	// https://github.com/golang/protobuf/issues/1121
	options := proto.MarshalOptions{Deterministic: true}
	return options.Marshal(msg)
}
