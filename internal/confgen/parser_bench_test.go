package confgen

import (
	"runtime"
	"testing"

	"github.com/tableauio/tableau/proto/tableaupb/unittestpb"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

// BenchmarkWellKnownFieldConstruction isolates the construction path used by
// nested well-known fields. The redundant_message case models the old parser,
// which allocated a dynamic message before ParseFieldValue replaced it with
// the parsed timestamp or duration value.
func BenchmarkWellKnownFieldConstruction(b *testing.B) {
	parser := newTableParserForTest()
	timeMD := (&unittestpb.PatchMergeConf_Time{}).ProtoReflect().Descriptor()
	fields := []struct {
		name  protoreflect.Name
		value string
	}{
		{name: "start", value: "2026-01-02 03:04:05"},
		{name: "expiry", value: "1h2m3s"},
	}

	for _, tc := range fields {
		field := parser.parseFieldDescriptor(timeMD.Fields().ByName(tc.name))
		b.Run(string(tc.name)+"/optimized", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				value, _, err := parser.parseFieldValue(field.fd, tc.value, field.opts.Prop)
				if err != nil {
					b.Fatal(err)
				}
				runtime.KeepAlive(value)
			}
		})
		b.Run(string(tc.name)+"/redundant_message", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				unused := dynamicpb.NewMessage(field.fd.Message())
				value, _, err := parser.parseFieldValue(field.fd, tc.value, field.opts.Prop)
				if err != nil {
					b.Fatal(err)
				}
				runtime.KeepAlive(unused)
				runtime.KeepAlive(value)
			}
		})
	}
}

// BenchmarkDynamicMessageConstruction compares allocating a dynamic message
// with resetting one for reuse. Reset is included as a guard against adding a
// pool that only moves the allocation cost into reset itself.
func BenchmarkDynamicMessageConstruction(b *testing.B) {
	md := (&unittestpb.PatchMergeConf_Time{}).ProtoReflect().Descriptor()
	secondsFD := md.Fields().ByName("start").Message().Fields().ByName("seconds")
	nanosFD := md.Fields().ByName("start").Message().Fields().ByName("nanos")

	b.Run("new_each", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			msg := dynamicpb.NewMessage(secondsFD.ContainingMessage())
			msg.Set(secondsFD, protoreflect.ValueOfInt64(1))
			msg.Set(nanosFD, protoreflect.ValueOfInt32(2))
			runtime.KeepAlive(msg)
		}
	})
	b.Run("reset_reuse", func(b *testing.B) {
		b.ReportAllocs()
		msg := dynamicpb.NewMessage(secondsFD.ContainingMessage())
		for i := 0; i < b.N; i++ {
			msg.Reset()
			msg.Set(secondsFD, protoreflect.ValueOfInt64(1))
			msg.Set(nanosFD, protoreflect.ValueOfInt32(2))
		}
		runtime.KeepAlive(msg)
	})
}
