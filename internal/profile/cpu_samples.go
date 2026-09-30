package profile

import (
	"errors"
	"os"
	"time"

	profiledata "github.com/google/pprof/profile"
	"github.com/tableauio/tableau/internal/x/xerrors"
)

func cpuTimeByLabel(filename, label string) (samples map[string]time.Duration, err error) {
	if filename == "" {
		return nil, nil
	}
	file, err := os.Open(filename)
	if err != nil {
		return nil, xerrors.Wrapf(err, "open CPU profile %s", filename)
	}
	defer func() {
		err = errors.Join(err, xerrors.Wrapf(file.Close(), "close CPU profile %s", filename))
	}()
	parsed, err := profiledata.Parse(file)
	if err != nil {
		return nil, xerrors.Wrapf(err, "parse CPU profile %s", filename)
	}
	cpuSampleIndex := -1
	for i, sampleType := range parsed.SampleType {
		if sampleType.Type == "cpu" && sampleType.Unit == "nanoseconds" {
			cpuSampleIndex = i
			break
		}
	}
	if cpuSampleIndex < 0 {
		return nil, xerrors.Newf("CPU profile %s has no cpu/nanoseconds samples", filename)
	}
	samples = make(map[string]time.Duration)
	for _, sample := range parsed.Sample {
		if cpuSampleIndex >= len(sample.Value) {
			continue
		}
		for _, key := range sample.Label[label] {
			samples[key] += time.Duration(sample.Value[cpuSampleIndex])
		}
	}
	return samples, nil
}
