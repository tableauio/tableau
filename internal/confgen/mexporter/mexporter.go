// mexporter is the message exporter package, which can export one
// single message to different formts: JSON, Text, and Bin.
package mexporter

import (
	"io/ioutil"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/proto/tableaupb"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
)

type messageExporter struct {
	name      string
	msg       proto.Message
	outputDir string
	outputOpt *options.ConfOutputOption
	wsOpts    *tableaupb.WorksheetOptions
}

func New(name string, msg proto.Message, outputDir string, outputOpt *options.ConfOutputOption, wsOpts *tableaupb.WorksheetOptions) *messageExporter {
	return &messageExporter{
		name:      name,
		msg:       msg,
		outputOpt: outputOpt,
		outputDir: filepath.Join(outputDir, outputOpt.Subdir),
		wsOpts:    wsOpts,
	}
}

// Export exports the message to the specified one or multiple forma(s).
func (x *messageExporter) Export() error {
	formats := format.OutputFormats
	if len(x.outputOpt.Formats) != 0 {
		formats = x.outputOpt.Formats
	}

	for _, fmt := range formats {
		err := x.export(fmt)
		if err != nil {
			return err
		}
	}
	return nil
}

// export marshals the message to the specified format and writes it to the
// specified file.
func (x *messageExporter) export(fmt format.Format) error {
	filename := x.name
	var out []byte
	var err error
	switch fmt {
	case format.JSON:
		filename += format.JSONExt
		out, err = x.marshalToJSON()
		if err != nil {
			return errors.Wrapf(err, "failed to export %s to JSON", x.name)
		}
	case format.Text:
		filename += format.TextExt
		out, err = x.marshalToText()
		if err != nil {
			return errors.Wrapf(err, "failed to export %s to Text", x.name)
		}
	case format.Bin:
		filename += format.BinExt
		out, err = x.marshalToBin()
		if err != nil {
			return errors.Wrapf(err, "failed to export %s to Bin", x.name)
		}
	default:
		return errors.Errorf("unknown output format: %v", fmt)
	}

	fpath := filepath.Join(x.outputDir, filename)
	err = ioutil.WriteFile(fpath, out, 0644)
	if err != nil {
		return errors.Wrapf(err, "failed to write file: %s", fpath)
	}
	// out.WriteTo(os.Stdout)
	log.Infof("%18s: %s", "generated conf", filename)
	return nil
}

func (x *messageExporter) marshalToJSON() (out []byte, err error) {
	if x.outputOpt.Pretty {
		opts := protojson.MarshalOptions{
			Multiline:       true,
			Indent:          "    ",
			EmitUnpopulated: x.outputOpt.EmitUnpopulated,
			UseProtoNames:   x.outputOpt.UseProtoNames,
			UseEnumNumbers:  x.outputOpt.UseEnumNumbers,
		}
		return opts.Marshal(x.msg)
	}
	return protojson.Marshal(x.msg)
}

func (x *messageExporter) marshalToText() (out []byte, err error) {
	if x.outputOpt.Pretty {
		opts := prototext.MarshalOptions{
			Multiline: true,
			Indent:    "    ",
		}
		return opts.Marshal(x.msg)
	}
	return prototext.Marshal(x.msg)
}

func (x *messageExporter) marshalToBin() (out []byte, err error) {
	return proto.Marshal(x.msg)
}
