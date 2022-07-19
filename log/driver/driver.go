package driver

import (
	"strings"

	"github.com/tableauio/tableau/log/core"
)

// refer: https://github.com/go-eden/slf4go/blob/master/slf_model.go

// Driver define the standard log print specification
type Driver interface {
	// Retrieve the name of current driver, like 'default', 'zap', 'logrus' ...
	Name() string

	// Print responsible of printing the standard Log
	Print(r *core.Record)

	// Retrieve log level of the specified logger,
	// it should return the lowest Level that could be print,
	// which can help invoker to decide whether prepare print or not.
	GetLevel(logger string) core.Level
}

var registeredDrivers = make(map[string]Driver)

// registered with the same name, the one registered last will take effect.
func RegisteDriver(driver Driver) {
	if driver == nil {
		panic("cannot register a nil Driver")
	}
	if driver.Name() == "" {
		panic("cannot register Driver with empty string result for Name()")
	}
	name := strings.ToLower(driver.Name())
	registeredDrivers[name] = driver
}

// GetDriver gets a registered Driver by driver name, or nil if no Driver is
// registered for the driver name.
//
// The driver name is expected to be lowercase.
func GetDriver(name string) Driver {
	return registeredDrivers[name]
}
