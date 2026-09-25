// Package profile writes CPU and memory profiles for generator runs.
package profile

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"

	"github.com/tableauio/tableau/internal/x/xerrors"
	"github.com/tableauio/tableau/internal/x/xfs"
	"github.com/tableauio/tableau/log"
)

// Start begins a CPU profile and returns a function that stops it and writes a
// post-GC memory profile. Profile files use name as their filename prefix. Only
// one CPU profile may run in a process at a time.
func Start(name, outputDir string) (func() error, error) {
	if err := os.MkdirAll(outputDir, xfs.DefaultDirPerm); err != nil {
		return nil, xerrors.Wrapf(err, "create profile directory %s", outputDir)
	}

	cpuPath := filepath.Join(outputDir, name+"-cpu.pprof")
	cpuFile, err := os.Create(cpuPath)
	if err != nil {
		return nil, xerrors.Wrapf(err, "create CPU profile %s", cpuPath)
	}
	if err := pprof.StartCPUProfile(cpuFile); err != nil {
		return nil, errors.Join(
			xerrors.Wrapf(err, "start CPU profile %s", cpuPath),
			xerrors.Wrapf(cpuFile.Close(), "close CPU profile %s", cpuPath),
			xerrors.Wrapf(os.Remove(cpuPath), "remove incomplete CPU profile %s", cpuPath),
		)
	}

	return func() error {
		pprof.StopCPUProfile()
		cpuCloseErr := xerrors.Wrapf(cpuFile.Close(), "close CPU profile %s", cpuPath)

		memPath := filepath.Join(outputDir, name+"-mem.pprof")
		memFile, err := os.Create(memPath)
		if err != nil {
			return errors.Join(cpuCloseErr, xerrors.Wrapf(err, "create memory profile %s", memPath))
		}

		runtime.GC()
		writeErr := xerrors.Wrapf(pprof.WriteHeapProfile(memFile), "write memory profile %s", memPath)
		memCloseErr := xerrors.Wrapf(memFile.Close(), "close memory profile %s", memPath)
		profileErr := errors.Join(cpuCloseErr, writeErr, memCloseErr)
		if profileErr == nil {
			log.Infof("wrote CPU profile: %s", cpuPath)
			log.Infof("wrote memory profile: %s", memPath)
		}
		return profileErr
	}, nil
}
