package main

import "testing"

func Benchmark_genProto(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = genProto("ERROR", "SIMPLE")
	}
}

func Benchmark_genConf(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = genConf("ERROR", "SIMPLE")
	}
}
