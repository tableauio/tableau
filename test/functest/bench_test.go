package functest

import "testing"

func Benchmark_genProto(b *testing.B) {
	for i := 0; i < b.N; i++ {
		genProto("ERROR")
	}
}

func Benchmark_genConf(b *testing.B) {
	for i := 0; i < b.N; i++ {
		genConf("ERROR")
	}
}
