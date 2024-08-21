package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tableauio/tableau"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/fs"
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
						"common/base.proto",
						"common/common.proto",
						"common/union.proto",
					},
					Formats: []format.Format{
						// format.Excel,
						format.CSV,
						format.XML,
						format.YAML,
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
					ProtoPaths: []string{"./_proto"},
					ProtoFiles: []string{"./_proto/*.proto"},
					Formats: []format.Format{
						// format.Excel,
						format.CSV,
						format.XML,
						format.YAML,
					},
					ExcludedProtoFiles: []string{
						"./_proto/xml__metasheet__metasheet.proto",
					},
				},
				Output: &options.ConfOutputOption{
					Pretty:          true,
					Formats:         []format.Format{format.JSON},
					EmitUnpopulated: true,
					DryRun:          options.DryRunPatch,
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

func EqualTextFile(fileExt string, oldDir, newDir string, startLineN int) error {
	files, err := os.ReadDir(oldDir)
	if err != nil {
		return err
	}
	for _, file := range files {
		if filepath.Ext(file.Name()) != fileExt {
			continue
		}
		oldPath := filepath.Join(oldDir, file.Name())
		absOldPath, err := filepath.Abs(oldPath)
		if err != nil {
			return err
		}
		oldfile, err := os.Open(oldPath)
		if err != nil {
			return err
		}

		newPath := filepath.Join(newDir, file.Name())
		absNewPath, err := filepath.Abs(newPath)
		if err != nil {
			return err
		}
		newfile, err := os.Open(newPath)
		if err != nil {
			return err
		}

		oscan := bufio.NewScanner(oldfile)
		nscan := bufio.NewScanner(newfile)

		ln := 0
		for {
			sok := oscan.Scan()
			dok := nscan.Scan()
			ln++
			if sok != dok {
				return fmt.Errorf("line count not equal: %s:%d -> %s:%d", absOldPath, ln, absNewPath, ln)
			}
			if !sok || !dok {
				break
			}
			if ln < startLineN {
				// as the first line is one line comment
				// (including dynamic version number), ignore it.
				continue
			}
			oldLine := string(oscan.Bytes())
			newLine := string(nscan.Bytes())
			if oldLine != newLine {
				return fmt.Errorf("line diff:\nold: %s:%d\n%s\nnew: %s:%d\n%s", 
				absOldPath, ln, oldLine, absNewPath, ln, newLine)
			}
		}
	}
	return nil
}
