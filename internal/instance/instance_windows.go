//go:build windows

package instance

import (
	"errors"
	"fmt"

	"golang.org/x/sys/windows"
)

const mutexName = `Local\PrinterOne.SingleInstance.v1`

// Guard owns the Windows mutex for the lifetime of the application.
type Guard struct{ handle windows.Handle }

// Acquire returns ErrAlreadyRunning when another PrinterOne process owns the
// mutex. It never terminates or otherwise interacts with that process.
func Acquire() (*Guard, error) {
	name, err := windows.UTF16PtrFromString(mutexName)
	if err != nil {
		return nil, fmt.Errorf("encode mutex name: %w", err)
	}
	handle, err := windows.CreateMutex(nil, false, name)
	if err != nil {
		if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
			if handle != 0 {
				windows.CloseHandle(handle)
			}
			return nil, ErrAlreadyRunning
		}
		return nil, fmt.Errorf("create instance mutex: %w", err)
	}
	return &Guard{handle: handle}, nil
}

func (g *Guard) Close() error {
	if g == nil || g.handle == 0 {
		return nil
	}
	err := windows.CloseHandle(g.handle)
	g.handle = 0
	return err
}
