// Package store provides functions to store a protobuf message to
// different formats: json, bin, and txt.
package store

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/xerrors"
	"google.golang.org/protobuf/proto"
)

// Store stores protobuf message to file in the specified directory and format.
// Available formats: JSON, Bin, and Text.
func Store(msg proto.Message, dir string, fmt format.Format, options ...Option) error {
	opts := ParseOptions(options...)
	var name string
	if opts.Name != "" {
		name = opts.Name // name specified explicitly
	} else {
		name = string(msg.ProtoReflect().Descriptor().Name())
	}
	filename := name
	var out []byte
	var err error
	switch fmt {
	case format.JSON:
		filename += format.JSONExt
		options := &MarshalOptions{
			Pretty:          opts.Pretty,
			EmitUnpopulated: opts.EmitUnpopulated,
			UseProtoNames:   opts.UseProtoNames,
			UseEnumNumbers:  opts.UseEnumNumbers,
		}
		out, err = MarshalToJSON(msg, options)
		if err != nil {
			return errors.Wrapf(err, "failed to export %s to JSON", name)
		}
	case format.Text:
		filename += format.TextExt
		out, err = MarshalToText(msg, opts.Pretty)
		if err != nil {
			return errors.Wrapf(err, "failed to export %s to Text", name)
		}
	case format.Bin:
		filename += format.BinExt
		out, err = MarshalToBin(msg)
		if err != nil {
			return errors.Wrapf(err, "failed to export %s to Bin", name)
		}
	default:
		return errors.Errorf("unknown output format: %v", fmt)
	}

	fpath := filepath.Join(dir, filename)
	// prepare dir
	if err := os.MkdirAll(filepath.Dir(fpath), 0700); err != nil {
		return xerrors.WrapKV(err, `failed to create dir "%s"`, filepath.Dir(fpath))
	}

	// write file
	err = os.WriteFile(fpath, out, 0644)
	if err != nil {
		return xerrors.Errorf(`write file "%s" failed: %s`, fpath, err)
	}
	// out.WriteTo(os.Stdout)
	log.Infof("%18s: %s", "generated conf", filename)
	return nil
}
