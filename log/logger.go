package log

type LoggerIface interface {
	// Debug uses fmt.Sprint to construct and log a message.
	Debug(args ...any)

	// Info uses fmt.Sprint to construct and log a message.
	Info(args ...any)

	// Warn uses fmt.Sprint to construct and log a message.
	Warn(args ...any)

	// Error uses fmt.Sprint to construct and log a message.
	Error(args ...any)

	// DPanic uses fmt.Sprint to construct and log a message. In development, the
	// logger then panics. (See DPanicLevel for details.)
	DPanic(args ...any)

	// Panic uses fmt.Sprint to construct and log a message, then panics.
	Panic(args ...any)

	// Fatal uses fmt.Sprint to construct and log a message, then calls os.Exit.
	Fatal(args ...any)

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

	// Debugw logs a message with some additional context. The variadic key-value
	// pairs are treated as they are in With.
	Debugw(msg string, keysAndValues ...any)

	// Infow logs a message with some additional context.
	Infow(msg string, keysAndValues ...any)

	// Warnw logs a message with some additional context.
	Warnw(msg string, keysAndValues ...any)

	// Errorw logs a message with some additional context.
	Errorw(msg string, keysAndValues ...any)

	// DPanicw logs a message with some additional context.
	DPanicw(msg string, keysAndValues ...any)

	// Panicw logs a message with some additional context.
	Panicw(msg string, keysAndValues ...any)

	// Fatalw logs a message with some additional context.
	Fatalw(msg string, keysAndValues ...any)
}
