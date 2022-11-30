package functest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tableauio/tableau"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/fs"
	"github.com/tableauio/tableau/internal/importer"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/options"
)

func genProto(logLevel string) error {
	// prepare output common dir
	outdir := "./_proto"
	err := os.MkdirAll(outdir, 0700)
	if err != nil {
		return fmt.Errorf("failed to create output dir: %v", err)
	}
	outCommDir := filepath.Join(outdir, "common")
	err = os.MkdirAll(outCommDir, 0700)
	if err != nil {
		return fmt.Errorf("failed to create output common dir: %v", err)
	}

	srcCommDir := "./proto/common"
	dirEntries, err := os.ReadDir(srcCommDir)
	if err != nil {
		return fmt.Errorf("read dir failed: %+v", err)
	}
	for _, entry := range dirEntries {
		if !entry.IsDir() {
			src := filepath.Join(srcCommDir, entry.Name())
			dst := filepath.Join(outCommDir, entry.Name())
			if err := fs.CopyFile(src, dst); err != nil {
				return fmt.Errorf("copy file failed: %+v", err)
			}
		}
	}

	return tableau.GenProto(
		"protoconf",
		"./testdata",
		outdir,
		options.Proto(
			&options.ProtoOption{
				Input: &options.ProtoInputOption{
					ProtoPaths: []string{outdir},
					ProtoFiles: []string{
						"common/common.proto",
					},
					Formats: []format.Format{
						// format.Excel,
						format.CSV,
						format.XML,
					},
					Header: &options.HeaderOption{
						Namerow: 1,
						Typerow: 2,
						Noterow: 3,
						Datarow: 4,
					},
				},
				Output: &options.ProtoOutputOption{
					FilenameWithSubdirPrefix: true,
					FileOptions: map[string]string{
						"go_package": "github.com/tableauio/tableau/test/functest/protoconf",
					},
				},
			},
		),
		options.Log(
			&log.Options{
				Level: logLevel,
				Mode:  "FULL",
			},
		),
		// options.Lang("zh"),
	)
}

func genConf(logLevel string) error {
	return tableau.GenConf(
		"protoconf",
		"./testdata",
		"./_conf",
		options.LocationName("Asia/Shanghai"),
		options.Conf(
			&options.ConfOption{
				Input: &options.ConfInputOption{
					ProtoPaths: []string{"./_proto", "."},
					ProtoFiles: []string{"./_proto/*.proto"},
					Formats: []format.Format{
						// format.Excel,
						format.CSV,
						format.XML,
					},
					ExcludedProtoFiles: []string{
						"./_proto/xml__metasheet__metasheet.proto",
					},
				},
				Output: &options.ConfOutputOption{
					Pretty:          true,
					Formats:         []format.Format{format.JSON},
					EmitUnpopulated: true,
				},
			},
		),
		options.Log(
			&log.Options{
				Level: logLevel,
				Mode:  "FULL",
			},
		),
		options.Lang("zh"),
	)
}

func rangeFilesByFormat(dir string, fmt format.Format, callback func(bookPath string) error) error {
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	// book name -> existence(bool)
	csvBooks := map[string]bool{}
	for _, entry := range dirEntries {
		if entry.IsDir() {
			// scan and generate subdir recursively
			subdir := filepath.Join(dir, entry.Name())
			err = rangeFilesByFormat(subdir, fmt, callback)
			if err != nil {
				return err
			}
			continue
		}
		fileFmt := format.Ext2Format(filepath.Ext(entry.Name()))
		if fileFmt != fmt {
			continue
		}
		switch fmt {
		case format.Excel:
			bookPath := filepath.Join(dir, entry.Name())
			if err := callback(bookPath); err != nil {
				return err
			}
		case format.CSV:
			bookName, _, err := importer.ParseCSVFilenamePattern(entry.Name())
			if err != nil {
				return err
			}
			if _, ok := csvBooks[bookName]; ok {
				// NOTE: multiple CSV files construct the same book.
				continue
			}
			csvBooks[bookName] = true
			if err := callback(importer.GenCSVBookFilenamePattern(dir, bookName)); err != nil {
				return err
			}
		default:
			return errors.New("unknown fommat: " + string(fmt))
		}
	}
	return nil
}
