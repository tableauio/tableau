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

func TestHeaderExplicitness(t *testing.T) {
	header := NewDefault().Proto.Input.Header
	if header.IsExplicit() {
		t.Fatal("the initialized header should be implicit")
	}
	header.DataRow = DefaultDataRow + 1
	if !header.IsExplicit() {
		t.Fatal("a customized header should be explicit")
	}

	opts := NewDefault()
	if err := yaml.Unmarshal([]byte("proto: {input: {header: {}}}"), opts); err != nil {
		t.Fatal(err)
	}
	if !opts.Proto.Input.Header.IsExplicit() {
		t.Fatal("an empty YAML header should be explicit even when merged over defaults")
	}
}
