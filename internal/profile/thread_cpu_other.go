//go:build !linux && !darwin && !windows

package profile

import "time"

// MeasureThreadCPUTime reports unavailable on unsupported platforms.
func MeasureThreadCPUTime() (time.Duration, bool) {
	return 0, false
}
