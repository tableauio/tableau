// Package profile measures generator work and writes CPU and memory profiles.
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

type session struct {
	cpuFile *os.File
	cpuPath string
	memPath string
}

// Files names the profiles produced by Capture. Empty paths mean profiling
// could not start and work ran without profile files.
type Files struct {
	CPU    string
	Memory string
}

// Capture runs work while collecting process CPU and post-GC memory profiles
// and returns their paths. If another CPU profile is active, Capture logs the
// conflict and runs work without writing profile files.
func Capture(name, outputDir string, work func() error) (files Files, err error) {
	s, startErr := start(name, outputDir)
	if startErr != nil {
		log.Warnf("profiling unavailable: %v", startErr)
		return Files{}, work()
	}
	files = Files{CPU: s.cpuPath, Memory: s.memPath}
	defer func() {
		err = errors.Join(err, s.stop())
	}()
	return files, work()
}

func start(name, outputDir string) (*session, error) {
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
	return &session{
		cpuFile: cpuFile,
		cpuPath: cpuPath,
		memPath: filepath.Join(outputDir, name+"-mem.pprof"),
	}, nil
}

func (s *session) stop() error {
	pprof.StopCPUProfile()
	cpuCloseErr := xerrors.Wrapf(s.cpuFile.Close(), "close CPU profile %s", s.cpuPath)

	memFile, err := os.Create(s.memPath)
	if err != nil {
		return errors.Join(cpuCloseErr, xerrors.Wrapf(err, "create memory profile %s", s.memPath))
	}
	runtime.GC()
	writeErr := xerrors.Wrapf(pprof.WriteHeapProfile(memFile), "write memory profile %s", s.memPath)
	memCloseErr := xerrors.Wrapf(memFile.Close(), "close memory profile %s", s.memPath)
	if err := errors.Join(cpuCloseErr, writeErr, memCloseErr); err != nil {
		return err
	}

	log.Infof("wrote CPU profile: %s", s.cpuPath)
	log.Infof("wrote memory profile: %s", s.memPath)
	return nil
}
