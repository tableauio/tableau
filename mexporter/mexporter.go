// Package mexporter can export a protobuf message to different formts: JSON,
// Text, and Bin.
package mexporter

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/xerrors"
	"google.golang.org/protobuf/proto"
)

// Exporter is designed for exporting a protobuf message to one or multiple
// files.
type Exporter struct {
	name      string
	msg       proto.Message
	outputDir string
	outputOpt *options.ConfOutputOption
}

// New creates a new protobuf message Exporter.
func New(name string, msg proto.Message, outputDir string, outputOpt *options.ConfOutputOption) *Exporter {
	return &Exporter{
		name:      name,
		msg:       msg,
		outputOpt: outputOpt,
		outputDir: filepath.Join(outputDir, outputOpt.Subdir),
	}
}

// Export exports the message to the specified one or multiple forma(s).
func (x *Exporter) Export() error {
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
func (x *Exporter) export(fmt format.Format) error {
	filename := x.name
	var out []byte
	var err error
	switch fmt {
	case format.JSON:
		filename += format.JSONExt
		options := &MarshalOptions{
			Pretty:          x.outputOpt.Pretty,
			EmitUnpopulated: x.outputOpt.EmitUnpopulated,
			UseProtoNames:   x.outputOpt.UseProtoNames,
			UseEnumNumbers:  x.outputOpt.UseEnumNumbers,
		}
		out, err = MarshalToJSON(x.msg, options)
		if err != nil {
			return errors.Wrapf(err, "failed to export %s to JSON", x.name)
		}
	case format.Text:
		filename += format.TextExt
		out, err = MarshalToText(x.msg, x.outputOpt.Pretty)
		if err != nil {
			return errors.Wrapf(err, "failed to export %s to Text", x.name)
		}
	case format.Bin:
		filename += format.BinExt
		out, err = MarshalToBin(x.msg)
		if err != nil {
			return errors.Wrapf(err, "failed to export %s to Bin", x.name)
		}
	default:
		return errors.Errorf("unknown output format: %v", fmt)
	}

	// prepare output dir
	if err := os.MkdirAll(x.outputDir, 0700); err != nil {
		return xerrors.WrapKV(err, "OutputDir", x.outputDir)
	}

	// write file
	fpath := filepath.Join(x.outputDir, filename)
	err = os.WriteFile(fpath, out, 0644)
	if err != nil {
		return errors.Wrapf(err, "failed to write file: %s", fpath)
	}
	// out.WriteTo(os.Stdout)
	log.Infof("%18s: %s", "generated conf", filename)
	return nil
}
