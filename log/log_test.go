package log

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tableauio/tableau/log/core"
	"github.com/tableauio/tableau/log/driver/zapdriver"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestDebug(t *testing.T) {
	type args struct {
		args []any
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
		{
			name: "test",
			args: args{
				args: []any{"xxx", 1, true},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Debug(tt.args.args...)
		})
	}
}

func TestInfow(t *testing.T) {
	type args struct {
		msg           string
		keysAndValues []any
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
		{
			name: "test",
			args: args{
				msg:           "infow test",
				keysAndValues: []any{"xxx", 1, "key2", true},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Infow(tt.args.msg, tt.args.keysAndValues...)
		})
	}
}

func TestLevel(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{
			name: "test",
			want: "DEBUG",
		},
	}
	err := Init(&Options{
		Level: "DEBUG",
		Mode:  "FULL",
	})
	assert.NoError(t, err)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Level(); got != tt.want {
				t.Errorf("Level() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_logs(t *testing.T) {
	args := []any{"xxx", 1, "key2", true}
	Info(args...)
	Warn(args...)
	Error(args...)

	// Panic(args...)
	Debugf("count: %d", 1)
	Infof("count: %d", 1)
	Warnf("count: %d", 1)
	Errorf("count: %d", 1)

	Debugw("test", args)
	Infow("test", args...)
	Warnw("test", args)
	Errorw("test", args)

	assert.Panics(t, func() {
		Panic(args...)
	})
	assert.Panics(t, func() {
		Panic(args...)
		Panicf("count: %d", 1)
	})
	assert.Panics(t, func() {
		Panicw("test", args)
	})
	// NOTE: we cannot test fatal, because it will exit the process.
	// assert.Panics(t, func() {
	// 	Fatal(args...)
	// })
	// assert.Panics(t, func() {
	// 	Fatalf("count: %d", 1)
	// })
	// assert.Panics(t, func() {
	// 	Fatalw("test", args)
	// })
}

func TestMain(m *testing.M) {
	defaultLogger = &Logger{
		level: core.DebugLevel,
		// driver: &defaultdriver.DefaultDriver{
		// 	CallerSkip: 1,
		// },
		driver: zapdriver.New(
			zap.NewDevelopmentConfig(),
			zap.AddCallerSkip(4),
			zap.WithFatalHook(zapcore.WriteThenPanic),
		),
	}
	os.Exit(m.Run())
}
