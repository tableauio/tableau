package core

import (
	"fmt"
	"strings"
)

// refer: https://github.com/go-eden/slf4go/blob/master/slf_model.go

// Fields represents attached fields of log
type Fields map[string]interface{}

// Record represent an log, contains all properties.
type Record struct {
	Level Level

	Format *string
	Args   []interface{}

	KVs       []interface{} // additional custom variadic key-value pairs
	CxtFields Fields        // caller's goroutine context fields
}

type Mode string

const (
	ModeSimple Mode = "SIMPLE"
	ModeFull   Mode = "FULL"
)

type SinkType int

const (
	SinkConsole SinkType = iota // default
	SinkFile
	SinkMulti
)

var sinkMap = map[string]SinkType{
	"":        SinkConsole,
	"CONSOLE": SinkConsole,
	"FILE":    SinkFile,
	"MULTI":   SinkMulti,
}

func GetSinkType(sink string) (SinkType, error) {
	sinkType, ok := sinkMap[strings.ToUpper(sink)]
	if !ok {
		return SinkConsole, fmt.Errorf("illegal sink: %s", sink)
	}
	return sinkType, nil
}
