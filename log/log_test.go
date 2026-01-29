package log

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_logs(t *testing.T) {
	Debugf("count: %d", 1)
	Infof("count: %d", 1)
	Warnf("count: %d", 1)
	Errorf("count: %d", 1)

	assert.Panics(t, func() {
		Panicf("count: %d", 1)
	})
	// NOTE: we cannot test fatal, because it will exit the process.
	// assert.Panics(t, func() {
	// 	Fatalf("count: %d", 1)
	// })
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
