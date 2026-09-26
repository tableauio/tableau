// Package profile measures generator work and writes CPU and memory profiles.
package profile

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sync"

	"github.com/tableauio/tableau/internal/x/xerrors"
	"github.com/tableauio/tableau/internal/x/xfs"
	"github.com/tableauio/tableau/log"
)

var processProfileMu sync.Mutex

// ExecuteWithStart runs work with optional process profiling and reports
// collected metrics after the profile is stopped. A profiler conflict disables
// profile files for this run without preventing the work itself.
func ExecuteWithStart(enabled bool, start func() (func() error, error), report func(), work func() error) (err error) {
	if !enabled {
		return work()
	}
	if report != nil {
		defer report()
	}
	stop, startErr := start()
	if startErr != nil {
		log.Warnf("profiling unavailable: %v", startErr)
		return work()
	}
	defer func() {
		err = errors.Join(err, stop())
	}()
	return work()
}

// Start begins a CPU profile and returns a function that stops it and writes a
// post-GC memory profile. Profile files use name as their filename prefix.
// Only one CPU profile may run in a process at a time.
func Start(name, outputDir string) (func() error, error) {
	// CPU profiling is process-global. Let one Tableau session own it while
	// concurrent or nested generators continue without profile files.
	if !processProfileMu.TryLock() {
		return nil, xerrors.New("another Tableau profile is active")
	}
	started := false
	defer func() {
		if !started {
			processProfileMu.Unlock()
		}
	}()

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
	started = true

	var once sync.Once
	var stopErr error
	return func() error {
		once.Do(func() {
			defer processProfileMu.Unlock()
			pprof.StopCPUProfile()
			cpuCloseErr := xerrors.Wrapf(cpuFile.Close(), "close CPU profile %s", cpuPath)

			memPath := filepath.Join(outputDir, name+"-mem.pprof")
			memFile, err := os.Create(memPath)
			if err != nil {
				stopErr = errors.Join(cpuCloseErr, xerrors.Wrapf(err, "create memory profile %s", memPath))
				return
			}

			runtime.GC()
			writeErr := xerrors.Wrapf(pprof.WriteHeapProfile(memFile), "write memory profile %s", memPath)
			memCloseErr := xerrors.Wrapf(memFile.Close(), "close memory profile %s", memPath)

			stopErr = errors.Join(cpuCloseErr, writeErr, memCloseErr)
			if stopErr == nil {
				log.Infof("wrote CPU profile: %s", cpuPath)
				log.Infof("wrote memory profile: %s", memPath)
			}
		})
		return stopErr
	}, nil
}
