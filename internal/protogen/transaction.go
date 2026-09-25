package protogen

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/tableauio/tableau/internal/x/xerrors"
	"github.com/tableauio/tableau/log"
)

// commit backs up affected files, publishes staged files, and drops stale files.
// A failed filesystem step restores the backups before returning.
func (out *protoOutput) commit(removeStale bool) error {
	files := maps.Clone(out.staged)
	var stale []string
	if removeStale {
		var err error
		stale, err = staleProtoFiles(out.dir, out.owners, out.imports)
		if err != nil {
			return err
		}
		for _, path := range stale {
			key, err := absoluteProtoPath(path)
			if err != nil {
				return err
			}
			files[key] = ""
		}
	}

	paths := slices.Sorted(maps.Keys(files))
	backups := make(map[string]string)
	var published []string
	fail := func(err error) error {
		return errors.Join(err, out.restore(backups, published))
	}

	for i, path := range paths {
		exists, err := existingGeneratedProto(path)
		if err != nil {
			return fail(err)
		}
		if !exists {
			continue
		}
		backup := filepath.Join(out.stageDir, fmt.Sprintf("backup-%d.tmp", i))
		if err := os.Rename(path, backup); err != nil {
			return fail(xerrors.WrapKV(err))
		}
		backups[path] = backup
	}
	for _, path := range paths {
		stage := files[path]
		if stage == "" {
			continue
		}
		if err := os.Rename(stage, path); err != nil {
			return fail(xerrors.WrapKV(err))
		}
		published = append(published, path)
	}
	for _, path := range stale {
		log.Infof("%15s: %s", "removed stale proto", path)
	}
	return nil
}

func (out *protoOutput) restore(backups map[string]string, published []string) error {
	var failures []error
	for _, path := range published {
		if err := os.Remove(path); err != nil {
			failures = append(failures, fmt.Errorf("remove new proto %s: %w", path, err))
		}
	}
	for path, backup := range backups {
		if err := os.Rename(backup, path); err != nil {
			failures = append(failures, fmt.Errorf("restore proto %s: %w", path, err))
		}
	}
	if len(failures) > 0 {
		out.keepStage = true
		failures = append(failures, fmt.Errorf("previous files remain in %s", out.stageDir))
	}
	return errors.Join(failures...)
}
