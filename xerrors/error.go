package xerrors

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/tableauio/tableau/log"
)

// WithStack annotates err with a stack trace at the point WithStack was called.
// If err is nil, WithStack returns nil.
func WithStack(err error) error {
	if err == nil {
		return nil
	}
	stackErr := fmt.Errorf("%s", CombineKV(Error, err))
	return errors.WithStack(stackErr)
}

// Errorw formats the key-value pairs as `[|key: value]...` string and
// returns the string as a value that satisfies error.
// Errorw also records the stack trace at the point it was called.
func ErrorKV(err error, keysAndValues ...interface{}) error {
	msg := CombineKV(keysAndValues...) + CombineKV(Error, err)
	return errors.Errorf(msg)
}

// WithMessageKV annotates err with the key-value pairs as `[|key: value]...` string.
// If err is nil, WithMessagef returns nil.
func WithMessageKV(err error, keysAndValues ...interface{}) error {
	if err == nil {
		return nil
	}
	return errors.WithMessage(err, CombineKV(keysAndValues...))
}

func CombineKV(keysAndValues ...interface{}) string {
	var msg string
	for i := 0; i < len(keysAndValues); i += 2 {
		if i == len(keysAndValues)-1 {
			log.DPanic("invalid Key-Value pairs: odd number")
			break
		}
		key, val := keysAndValues[i], keysAndValues[i+1]
		msg += fmt.Sprintf("|%s: %s", key, val)
	}
	return msg
}

func ExtractDesc(err error) *Desc {
	desc := NewDesc()
	splits := strings.Split(err.Error(), "|")
	for _, s := range splits {
		kv := strings.SplitN(s, ":", 2)
		if len(kv) == 2 {
			key, val := strings.Trim(kv[0], " :"), strings.Trim(kv[1], " :")
			desc.UpdateField(key, val)
		}
	}
	return desc
}
