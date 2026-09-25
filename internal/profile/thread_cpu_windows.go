//go:build windows

package profile

import (
	"syscall"
	"time"
	"unsafe"
)

var (
	kernel32              = syscall.NewLazyDLL("kernel32.dll")
	getCurrentThread      = kernel32.NewProc("GetCurrentThread")
	getCurrentThreadTimes = kernel32.NewProc("GetThreadTimes")
)

// MeasureThreadCPUTime returns user plus system CPU time consumed by the
// current OS thread.
func MeasureThreadCPUTime() (time.Duration, bool) {
	thread, _, _ := getCurrentThread.Call()
	if thread == 0 {
		return 0, false
	}
	var created, exited, kernel, user syscall.Filetime
	ok, _, _ := getCurrentThreadTimes.Call(
		thread,
		uintptr(unsafe.Pointer(&created)),
		uintptr(unsafe.Pointer(&exited)),
		uintptr(unsafe.Pointer(&kernel)),
		uintptr(unsafe.Pointer(&user)),
	)
	if ok == 0 {
		return 0, false
	}
	kernelTicks := uint64(kernel.HighDateTime)<<32 | uint64(kernel.LowDateTime)
	userTicks := uint64(user.HighDateTime)<<32 | uint64(user.LowDateTime)
	return time.Duration((kernelTicks + userTicks) * 100), true
}
