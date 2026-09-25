//go:build linux || darwin

package profile

import (
	"time"

	"golang.org/x/sys/unix"
)

// MeasureThreadCPUTime returns user plus system CPU time consumed by the
// current OS thread.
func MeasureThreadCPUTime() (time.Duration, bool) {
	var value unix.Timespec
	if err := unix.ClockGettime(unix.CLOCK_THREAD_CPUTIME_ID, &value); err != nil {
		return 0, false
	}
	return time.Duration(value.Sec)*time.Second + time.Duration(value.Nsec), true
}
