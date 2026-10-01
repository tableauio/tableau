package protogen

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/proto/tableaupb/internalpb"
)

func TestProfilingDisabled(t *testing.T) {
	gen := NewGeneratorWithOptions("", t.TempDir(), t.TempDir(), options.NewDefault())
	if value, ok := pprof.Label(gen.ctx, "generator"); ok {
		t.Fatalf("generator label = %q, true; want profiling disabled", value)
	}
}

func TestPrepareRunFailureDoesNotWriteProfiles(t *testing.T) {
	blocked := filepath.Join(t.TempDir(), "not-a-directory")
	require.NoError(t, os.WriteFile(blocked, []byte("occupied"), 0o600))
	opts := options.NewDefault()
	opts.Profiling = true
	opts.Proto.Output.Subdir = "profiles"
	gen := NewGeneratorWithOptions("", t.TempDir(), blocked, opts)
	require.Error(t, gen.Generate())
	require.NoDirExists(t, filepath.Join(blocked, "profiles"))
	require.NoFileExists(t, filepath.Join(blocked, "profiles", "protogen-cpu.pprof"))
	require.NoFileExists(t, filepath.Join(blocked, "profiles", "protogen-mem.pprof"))
}

func TestNestedMetricsPreserveOperationLabels(t *testing.T) {
	gen := &Generator{ctx: context.Background(), profiling: true}
	sheet := book.NewTableSheet("Sheet", [][]string{{"value"}})
	sheet.Meta = &internalpb.Metasheet{}
	err := gen.measureOperation(gen.ctx, "generation", func(ctx context.Context) error {
		done := make(chan error, 1)
		go func() {
			done <- gen.measureOperation(ctx, "book_export", func(ctx context.Context) error {
				// Sheet scopes must preserve the enclosing operation labels.
				want := map[string]string{
					"operation.generation":  "generation",
					"operation.book_export": "book_export",
					"name":                  "book_export",
				}
				for key, value := range want {
					got, ok := pprof.Label(ctx, key)
					if !ok || got != value {
						return fmt.Errorf("label %s = %q, %t; want %q", key, got, ok, value)
					}
				}
				return gen.measureSheet(ctx, "Book", firstPass, sheet, func() error {
					// Capture inherited labels from a child goroutine while
					// the sheet scope is active.
					profile := pprof.Lookup("goroutine")
					var output strings.Builder
					if err := profile.WriteTo(&output, 1); err != nil {
						return err
					}
					for _, label := range []string{
						`"operation.generation":"generation"`,
						`"operation.book_export":"book_export"`,
						`"sheet_key":"Book#Sheet (first-pass)"`,
					} {
						if !strings.Contains(output.String(), label) {
							return fmt.Errorf("goroutine profile missing %s", label)
						}
					}
					return nil
				})
			})
		}()
		return <-done
	})
	require.NoError(t, err)
}

func TestGenerateWritesProfiles(t *testing.T) {
	inputDir := t.TempDir()
	outputDir := t.TempDir()
	opts := options.NewDefault()
	opts.Profiling = true
	opts.Proto.Output.Subdir = "profiles"
	gen := NewGeneratorWithOptions("", inputDir, outputDir, opts)
	if value, ok := pprof.Label(gen.ctx, "generator"); !ok || value != "protogen" {
		t.Fatalf("generator label = %q, %t; want protogen, true", value, ok)
	}

	if err := gen.Generate(); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"protogen-cpu.pprof", "protogen-mem.pprof"} {
		info, err := os.Stat(filepath.Join(outputDir, "profiles", name))
		if err != nil {
			t.Errorf("stat profile %s: %v", name, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("profile %s is empty", name)
		}
	}
}
