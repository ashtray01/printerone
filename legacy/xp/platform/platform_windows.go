//go:build windows
// +build windows

package platform

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

const (
	hkeyCurrentUser    = 0x80000001
	keyQueryValue      = 0x0001
	keySetValue        = 0x0002
	regSZ              = 1
	errorSuccess       = 0
	errorFileNotFound  = 2
	errorAlreadyExists = 183
)

var (
	kernel32            = syscall.NewLazyDLL("kernel32.dll")
	advapi32            = syscall.NewLazyDLL("advapi32.dll")
	procCreateMutex     = kernel32.NewProc("CreateMutexW")
	procRegOpenKeyEx    = advapi32.NewProc("RegOpenKeyExW")
	procRegCreateKeyEx  = advapi32.NewProc("RegCreateKeyExW")
	procRegQueryValueEx = advapi32.NewProc("RegQueryValueExW")
	procRegSetValueEx   = advapi32.NewProc("RegSetValueExW")
	procRegDeleteValue  = advapi32.NewProc("RegDeleteValueW")
	procRegCloseKey     = advapi32.NewProc("RegCloseKey")
)

type Instance struct{ handle syscall.Handle }

func AcquireInstance() (*Instance, error) {
	name, _ := syscall.UTF16PtrFromString("Local\\PrinterOne.XP.SingleInstance.v1")
	handle, _, callErr := procCreateMutex.Call(0, 0, uintptr(unsafe.Pointer(name)))
	if handle == 0 {
		return nil, callErr
	}
	if errno(callErr) == errorAlreadyExists {
		_ = syscall.CloseHandle(syscall.Handle(handle))
		return nil, errors.New("PrinterOne is already running")
	}
	return &Instance{handle: syscall.Handle(handle)}, nil
}

func (i *Instance) Close() {
	if i != nil && i.handle != 0 {
		_ = syscall.CloseHandle(i.handle)
		i.handle = 0
	}
}

const startupPath = `Software\Microsoft\Windows\CurrentVersion\Run`
const startupName = "PrinterOne-XP"

func StartupCommand() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	return `"` + path + `"`, nil
}

func StartupEnabled() (bool, error) {
	key, result := openKey(keyQueryValue)
	if result == errorFileNotFound {
		return false, nil
	}
	if result != errorSuccess {
		return false, syscall.Errno(result)
	}
	defer procRegCloseKey.Call(key)
	name, _ := syscall.UTF16PtrFromString(startupName)
	var typ, size uint32
	result, _, _ = procRegQueryValueEx.Call(key, uintptr(unsafe.Pointer(name)), 0, uintptr(unsafe.Pointer(&typ)), 0, uintptr(unsafe.Pointer(&size)))
	if result == errorFileNotFound {
		return false, nil
	}
	if result != errorSuccess {
		return false, syscall.Errno(result)
	}
	buf := make([]uint16, size/2+1)
	result, _, _ = procRegQueryValueEx.Call(key, uintptr(unsafe.Pointer(name)), 0, uintptr(unsafe.Pointer(&typ)), uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if result != errorSuccess {
		return false, syscall.Errno(result)
	}
	want, err := StartupCommand()
	return err == nil && typ == regSZ && strings.EqualFold(syscall.UTF16ToString(buf), want), err
}

func ConfigureStartup(enabled bool) error {
	name, _ := syscall.UTF16PtrFromString(startupName)
	if !enabled {
		key, result := openKey(keySetValue)
		if result == errorFileNotFound {
			return nil
		}
		if result != errorSuccess {
			return syscall.Errno(result)
		}
		defer procRegCloseKey.Call(key)
		result, _, _ = procRegDeleteValue.Call(key, uintptr(unsafe.Pointer(name)))
		if result != errorSuccess && result != errorFileNotFound {
			return syscall.Errno(result)
		}
		return nil
	}
	path, _ := syscall.UTF16PtrFromString(startupPath)
	var key uintptr
	var disposition uint32
	result, _, _ := procRegCreateKeyEx.Call(hkeyCurrentUser, uintptr(unsafe.Pointer(path)), 0, 0, 0, keySetValue, 0, uintptr(unsafe.Pointer(&key)), uintptr(unsafe.Pointer(&disposition)))
	if result != errorSuccess {
		return syscall.Errno(result)
	}
	defer procRegCloseKey.Call(key)
	command, err := StartupCommand()
	if err != nil {
		return err
	}
	value, _ := syscall.UTF16FromString(command)
	result, _, _ = procRegSetValueEx.Call(key, uintptr(unsafe.Pointer(name)), 0, regSZ, uintptr(unsafe.Pointer(&value[0])), uintptr(len(value)*2))
	if result != errorSuccess {
		return fmt.Errorf("write startup registry value: %v", syscall.Errno(result))
	}
	return nil
}

func openKey(access uintptr) (uintptr, uintptr) {
	path, _ := syscall.UTF16PtrFromString(startupPath)
	var key uintptr
	result, _, _ := procRegOpenKeyEx.Call(hkeyCurrentUser, uintptr(unsafe.Pointer(path)), 0, access, uintptr(unsafe.Pointer(&key)))
	return key, result
}

func errno(err error) syscall.Errno {
	if value, ok := err.(syscall.Errno); ok {
		return value
	}
	return 0
}
