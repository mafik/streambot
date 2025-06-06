package main

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32             = syscall.MustLoadDLL("user32.dll")
	procEnumWindows    = user32.MustFindProc("EnumWindows")
	procGetWindowTextW = user32.MustFindProc("GetWindowTextW")
)

func EnumWindows(enumFunc uintptr, lparam uintptr) (err error) {
	r1, _, e1 := syscall.Syscall(procEnumWindows.Addr(), 2, uintptr(enumFunc), uintptr(lparam), 0)
	if r1 == 0 {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func GetWindowText(hwnd syscall.Handle, str *uint16, maxCount int32) (len int32, err error) {
	r0, _, e1 := syscall.Syscall(procGetWindowTextW.Addr(), 3, uintptr(hwnd), uintptr(unsafe.Pointer(str)), uintptr(maxCount))
	len = int32(r0)
	if len == 0 {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func GetWindowTitle(hwnd windows.HWND) (string, error) {
	b := make([]uint16, 200)
	_, err := GetWindowText(syscall.Handle(hwnd), &b[0], int32(len(b)))
	if err != nil {
		return "", err
	}
	return syscall.UTF16ToString(b), nil
}

func FindWindow(title string) (windows.HWND, error) {
	var hwnd windows.HWND
	cb := syscall.NewCallback(func(h syscall.Handle, p uintptr) uintptr {
		windowTitle, err := GetWindowTitle(windows.HWND(h))
		if err != nil {
			// ignore the error
			return 1 // continue enumeration
		}

		if strings.Contains(windowTitle, title) {
			// note the window
			hwnd = windows.HWND(h)
			return 0 // stop enumeration
		}
		return 1 // continue enumeration
	})
	EnumWindows(cb, 0)
	if hwnd == 0 {
		return 0, fmt.Errorf("no window with title '%s' found", title)
	}
	return hwnd, nil
}
