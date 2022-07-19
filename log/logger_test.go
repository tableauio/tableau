package log

import (
	"testing"

	"github.com/tableauio/tableau/log/core"
	"github.com/tableauio/tableau/log/driver/defaultdriver"
)

func TestDefaultLogger_Debugf(t *testing.T) {
	type args struct {
		format string
		args   []interface{}
	}
	tests := []struct {
		name string
		l    *Logger
		args args
	}{
		// TODO: Add test cases.
		{
			name: "test",
			l: &Logger{
				level:  core.DebugLevel,
				driver: &defaultdriver.DefaultDriver{},
			},
			args: args{
				format: "format: %s, %d",
				args:   []interface{}{"haha", 3},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.l.Debugf(tt.args.format, tt.args.args...)
		})
	}
}
