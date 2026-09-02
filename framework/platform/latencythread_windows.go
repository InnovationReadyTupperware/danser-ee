//go:build windows

package platform

import (
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

const avrtPriorityHigh = 1

var (
	avrt                                = windows.NewLazySystemDLL("avrt.dll")
	procAvSetMmThreadCharacteristicsW   = avrt.NewProc("AvSetMmThreadCharacteristicsW")
	procAvSetMmThreadPriority           = avrt.NewProc("AvSetMmThreadPriority")
	procAvRevertMmThreadCharacteristics = avrt.NewProc("AvRevertMmThreadCharacteristics")
)

// BeginLatencySensitiveThread registers the calling thread with Windows'
// multimedia scheduler's Games task. The returned cleanup must be called once
// from the same OS thread.
func BeginLatencySensitiveThread() (func() error, error) {
	taskName, err := windows.UTF16PtrFromString("Games")
	if err != nil {
		return nil, fmt.Errorf("encode MMCSS task name: %w", err)
	}

	var taskIndex uint32
	handle, _, callErr := procAvSetMmThreadCharacteristicsW.Call(
		uintptr(unsafe.Pointer(taskName)),
		uintptr(unsafe.Pointer(&taskIndex)),
	)
	if handle == 0 {
		return nil, windowsCallError("register with MMCSS Games task", callErr)
	}

	prioritySet, _, callErr := procAvSetMmThreadPriority.Call(handle, avrtPriorityHigh)
	if prioritySet == 0 {
		_, _, _ = procAvRevertMmThreadCharacteristics.Call(handle)
		return nil, windowsCallError("set MMCSS thread priority", callErr)
	}

	return func() error {
		reverted, _, revertErr := procAvRevertMmThreadCharacteristics.Call(handle)
		if reverted == 0 {
			return windowsCallError("leave MMCSS Games task", revertErr)
		}

		return nil
	}, nil
}

func windowsCallError(operation string, err error) error {
	if err == nil || errors.Is(err, windows.ERROR_SUCCESS) {
		return errors.New(operation + " failed")
	}

	return fmt.Errorf("%s: %w", operation, err)
}
