package zapdriver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau/log/core"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type countingSink struct {
	writes int
	syncs  int
}

func (s *countingSink) Write(data []byte) (int, error) {
	s.writes++
	return len(data), nil
}

func (s *countingSink) Sync() error {
	s.syncs++
	return nil
}

func TestZapDriverSyncsExplicitly(t *testing.T) {
	sink := &countingSink{}
	encoder := zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
	logger := zap.New(zapcore.NewCore(encoder, sink, zap.DebugLevel))
	driver := NewWithLogger(zap.NewAtomicLevelAt(zap.DebugLevel), logger)

	driver.Print(&core.Record{Level: core.InfoLevel, Format: new(string), Args: []any{"message"}})
	require.Equal(t, 1, sink.writes)
	assert.Zero(t, sink.syncs)

	require.NoError(t, driver.Sync())
	assert.Equal(t, 1, sink.syncs)
}
