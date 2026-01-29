package log

type Options struct {
	// Log mode: SIMPLE, FULL.
	//
	// Default: "FULL".
	Mode string
	// Log level: DEBUG, INFO, WARN, ERROR.
	//
	// Default: "INFO".
	Level string
	// Log filename: set this if you want to write log messages to files.
	//
	// Default: "".
	Filename string
	// Log sink: CONSOLE, FILE, and MULTI.
	//
	// Default: "CONSOLE".
	Sink string
}
