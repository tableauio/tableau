package protogen

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/tableauio/tableau/internal/importer"
	"github.com/tableauio/tableau/internal/x/xerrors"
	"github.com/tableauio/tableau/log"
)

// importedProtoPaths expands configured imports so they are never swept or
// replaced by generated output.
func importedProtoPaths(patterns []string) (map[string]bool, error) {
	paths := make(map[string]bool)
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, xerrors.WrapKV(err)
		}
		for _, match := range matches {
			path, err := absoluteProtoPath(match)
			if err != nil {
				return nil, err
			}
			paths[path] = true
		}
	}
	return paths, nil
}

func (gen *Generator) beginRun() error {
	if gen.preserveStage {
		return xerrors.Newf("previous proto files still need recovery from %s", gen.stageDir)
	}
	gen.stageDir = ""
	protected, err := importedProtoPaths(gen.InputOpt.ProtoFiles)
	if err != nil {
		return err
	}
	gen.collector = xerrors.NewCollector(gen.ErrorLimitOpt.MaxErrors)
	gen.registryWithGeneratedOnce = sync.Once{}
	gen.protoRegistryFilesWithGenerated = nil
	gen.cacheMu.Lock()
	gen.cachedImporters = make(map[string]importer.Importer)
	gen.cacheMu.Unlock()
	gen.generatedMu.Lock()
	gen.generatedProtoFiles = make(map[string]string)
	gen.stagedProtoFiles = make(map[string]string)
	gen.protectedProtoFiles = protected
	gen.generatedMu.Unlock()
	return nil
}

func (gen *Generator) startStaging() error {
	outdir := filepath.Join(gen.OutputDir, gen.OutputOpt.Subdir)
	stageDir, err := os.MkdirTemp(outdir, ".tableau-stage-")
	if err != nil {
		return xerrors.WrapKV(err, xerrors.KeyOutdir, outdir)
	}
	gen.stageDir = stageDir
	return nil
}

func (gen *Generator) discardStaging() {
	if gen.stageDir == "" || gen.preserveStage {
		return
	}
	if err := os.RemoveAll(gen.stageDir); err != nil {
		log.Warnf("failed to remove temporary proto files in %s: %v", gen.stageDir, err)
	}
	gen.stageDir = ""
}

func (gen *Generator) stageProtoFile(path string, parts ...[]byte) (err error) {
	f, err := os.CreateTemp(gen.stageDir, "proto-*.tmp")
	if err != nil {
		return xerrors.WrapKV(err)
	}
	defer func() {
		if err != nil {
			_ = f.Close()
			_ = os.Remove(f.Name())
		}
	}()
	for _, part := range parts {
		if _, err = f.Write(part); err != nil {
			return xerrors.WrapKV(err)
		}
	}
	if err = f.Close(); err != nil {
		return xerrors.WrapKV(err)
	}
	key, err := absoluteProtoPath(path)
	if err != nil {
		return err
	}
	gen.generatedMu.Lock()
	gen.stagedProtoFiles[key] = f.Name()
	gen.generatedMu.Unlock()
	return nil
}

// commitOutputs replaces generated files and removes stale files only after
// parsing and rendering have succeeded. If a filesystem step fails, it restores
// the previous files from backups in the staging directory.
func (gen *Generator) commitOutputs(removeStale bool) error {
	gen.generatedMu.Lock()
	generated := make(map[string]bool, len(gen.generatedProtoFiles))
	for path := range gen.generatedProtoFiles {
		generated[path] = true
	}
	staged := maps.Clone(gen.stagedProtoFiles)
	protected := maps.Clone(gen.protectedProtoFiles)
	gen.generatedMu.Unlock()

	var stale []string
	if removeStale {
		var err error
		stale, err = staleProtoFiles(filepath.Join(gen.OutputDir, gen.OutputOpt.Subdir), generated, protected)
		if err != nil {
			return err
		}
	}

	affected := make(map[string]bool, len(staged)+len(stale))
	for path := range staged {
		affected[path] = true
	}
	for _, path := range stale {
		key, err := absoluteProtoPath(path)
		if err != nil {
			return err
		}
		affected[key] = true
	}
	paths := slices.Sorted(maps.Keys(affected))
	backups := make(map[string]string)
	var promoted []string
	rollback := func(cause error) error {
		var rollbackErrors []error
		for _, path := range promoted {
			if err := os.Remove(path); err != nil {
				rollbackErrors = append(rollbackErrors, fmt.Errorf("remove new proto %s: %w", path, err))
			}
		}
		for _, path := range paths {
			if backup, ok := backups[path]; ok {
				if err := os.Rename(backup, path); err != nil {
					rollbackErrors = append(rollbackErrors, fmt.Errorf("restore proto %s: %w", path, err))
				}
			}
		}
		if len(rollbackErrors) > 0 {
			gen.preserveStage = true
			rollbackErrors = append(rollbackErrors, fmt.Errorf("previous files remain in %s", gen.stageDir))
		}
		return errors.Join(append([]error{cause}, rollbackErrors...)...)
	}

	for i, path := range paths {
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return rollback(xerrors.WrapKV(err))
		}
		if !info.Mode().IsRegular() {
			return rollback(xerrors.Newf("proto output is not a regular file: %s", path))
		}
		generated, err := isGeneratedProtoFile(path)
		if err != nil {
			return rollback(err)
		}
		if !generated {
			return rollback(xerrors.Newf("refusing to overwrite non-generated proto file: %s", path))
		}
		backup := filepath.Join(gen.stageDir, fmt.Sprintf("backup-%d.tmp", i))
		if err := os.Rename(path, backup); err != nil {
			return rollback(xerrors.WrapKV(err))
		}
		backups[path] = backup
	}
	for _, path := range paths {
		stage, ok := staged[path]
		if !ok {
			continue
		}
		if err := os.Rename(stage, path); err != nil {
			return rollback(xerrors.WrapKV(err))
		}
		promoted = append(promoted, path)
	}
	for _, path := range stale {
		log.Infof("%15s: %s", "removed stale proto", path)
	}
	return nil
}
