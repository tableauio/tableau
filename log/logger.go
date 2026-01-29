package log

type LoggerIface interface {
	// Debugf uses fmt.Sprintf to log a templated message.
	Debugf(format string, args ...any)

	// Infof uses fmt.Sprintf to log a templated message.
	Infof(format string, args ...any)

	// Warnf uses fmt.Sprintf to log a templated message.
	Warnf(format string, args ...any)

	// Errorf uses fmt.Sprintf to log a templated message.
	Errorf(format string, args ...any)

	// DPanicf uses fmt.Sprintf to log a templated message. In development, the
	// logger then panics. (See DPanicLevel for details.)
	DPanicf(format string, args ...any)

	// Panicf uses fmt.Sprintf to log a templated message, then panics.
	Panicf(format string, args ...any)

	// Fatalf uses fmt.Sprintf to log a templated message, then calls os.Exit.
	Fatalf(format string, args ...any)
}
