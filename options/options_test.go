package options

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestProfilingDisabledByDefault(t *testing.T) {
	opts := NewDefault()
	if opts.Profiling {
		t.Fatal("profiling is enabled by default")
	}
}

func TestProfilingExcludedFromYAML(t *testing.T) {
	opts := NewDefault()
	opts.Profiling = true
	data, err := yaml.Marshal(opts)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "profiling") {
		t.Fatalf("runtime profiling option was serialized:\n%s", data)
	}
}
