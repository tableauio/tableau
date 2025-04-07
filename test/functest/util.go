package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tableauio/tableau"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/x/xfs"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/options"
)

func genProto(logLevel, logMode string) error {
	// prepare output common dir
	defaultOutdir := "./_proto/default"
	err := os.MkdirAll(defaultOutdir, xfs.DefaultDirPerm)
	if err != nil {
		return fmt.Errorf("failed to create output dir: %v", err)
	}
	outCommDir := filepath.Join(defaultOutdir, "common")
	err = os.MkdirAll(outCommDir, xfs.DefaultDirPerm)
	if err != nil {
		return fmt.Errorf("failed to create output common dir: %v", err)
	}

	srcCommDir := "./proto/default/common"
	dirEntries, err := os.ReadDir(srcCommDir)
	if err != nil {
		return fmt.Errorf("read dir failed: %+v", err)
	}
	for _, entry := range dirEntries {
		if !entry.IsDir() {
			src := filepath.Join(srcCommDir, entry.Name())
			dst := filepath.Join(outCommDir, entry.Name())
			if err := xfs.CopyFile(src, dst); err != nil {
				return fmt.Errorf("copy file failed: %+v", err)
			}
		}
	}

	err = tableau.GenProto(
		"protoconf",
		"./testdata/default",
		defaultOutdir,
		options.Proto(
			&options.ProtoOption{
				Input: &options.ProtoInputOption{
					ProtoPaths: []string{defaultOutdir},
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
						NameRow: 1,
						TypeRow: 2,
						NoteRow: 3,
						DataRow: 4,
						Sep:     ",",
						Subsep:  ":",
					},
					Subdirs: []string{"excel", "xml", "yaml"},
				},
				Output: &options.ProtoOutputOption{
					FilenameWithSubdirPrefix: true,
					FileOptions: map[string]string{
						"go_package": "github.com/tableauio/tableau/test/functest/protoconf",
					},
					EnumValueWithPrefix: true,
				},
			},
		),
		options.Log(
			&log.Options{
				Level: logLevel,
				Mode:  logMode,
			},
		),
		options.Acronyms(map[string]string{
			"K8s":          "k8s",
			"APIV3":        "apiv3",
			`(\d)[vV](\d)`: "${1}v${2}",
		}),
		// options.Lang("zh"),
	)
	if err != nil {
		return err
	}

	customOutdir := "./_proto/custom"
	return tableau.GenProto(
		"protoconf",
		"./testdata/custom",
		customOutdir,
		options.Proto(
			&options.ProtoOption{
				Input: &options.ProtoInputOption{
					ProtoPaths: []string{defaultOutdir},
					Formats: []format.Format{
						format.CSV,
					},
					Header: &options.HeaderOption{
						NameRow:  1,
						TypeRow:  1,
						NoteRow:  1,
						DataRow:  2,
						NameLine: 2,
						TypeLine: 3,
						Sep:      ",",
						Subsep:   ":",
					},
					Subdirs: []string{"excel"},
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
				Mode:  logMode,
			},
		),
	)
}

func genConf(logLevel, logMode string) error {
	err := tableau.GenConf(
		"protoconf",
		"./testdata/default",
		"./_conf/default",
		options.LocationName("Asia/Shanghai"),
		options.Conf(
			&options.ConfOption{
				Input: &options.ConfInputOption{
					ProtoPaths: []string{"./_proto/default/"},
					ProtoFiles: []string{"./_proto/default/*.proto"},
					Formats: []format.Format{
						// format.Excel,
						format.CSV,
						format.XML,
						format.YAML,
					},
					ExcludedProtoFiles: []string{
						"./_proto/default/xml__metasheet__metasheet.proto",
					},
				},
				Output: &options.ConfOutputOption{
					Pretty:          true,
					Formats:         []format.Format{format.JSON},
					EmitUnpopulated: true,
					EmitTimezones:    true,
					// DryRun:          options.DryRunPatch,
				},
			},
		),
		options.Log(
			&log.Options{
				Level: logLevel,
				Mode:  logMode,
			},
		),
		options.Lang("zh"),
	)
	if err != nil {
		return err
	}

	return tableau.GenConf(
		"protoconf",
		"./testdata/custom",
		"./_conf/custom",
		options.LocationName("Asia/Shanghai"),
		options.Conf(
			&options.ConfOption{
				Input: &options.ConfInputOption{
					ProtoPaths: []string{"./_proto/custom/"},
					ProtoFiles: []string{"./_proto/custom/*.proto"},
					Formats: []format.Format{
						// format.Excel,
						format.CSV,
						format.XML,
						format.YAML,
					},
				},
				Output: &options.ConfOutputOption{
					Pretty:          true,
					Formats:         []format.Format{format.JSON},
					EmitUnpopulated: true,
					// DryRun:          options.DryRunPatch,
				},
			},
		),
		options.Log(
			&log.Options{
				Level: logLevel,
				Mode:  logMode,
			},
		),
		options.Lang("zh"),
	)
}

func EqualTextFile(fileExt string, oldDir, newDir string, startLineN int) error {
	dirEntries, err := os.ReadDir(oldDir)
	if err != nil {
		return err
	}
	for _, entry := range dirEntries {
		if entry.IsDir() {
			subOldDir := filepath.Join(oldDir, entry.Name())
			subNewDir := filepath.Join(newDir, entry.Name())
			err := EqualTextFile(fileExt, subOldDir, subNewDir, startLineN)
			if err != nil {
				return err
			}
			continue
		} else if filepath.Ext(entry.Name()) != fileExt {
			continue
		}
		oldPath := filepath.Join(oldDir, entry.Name())
		absOldPath, err := filepath.Abs(oldPath)
		if err != nil {
			return err
		}
		oldfile, err := os.Open(oldPath)
		if err != nil {
			return err
		}

		newPath := filepath.Join(newDir, entry.Name())
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
