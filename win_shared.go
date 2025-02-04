//go:build windows
package main

import (
	"golang.org/x/sys/windows"
)

const (
	SW_HIDE = 0
	SW_SHOW = 5
	
	WS_VISIBLE = 0x10000000
	GWL_STYLE  = -16
)

var (
	kernel32                 = windows.NewLazySystemDLL("kernel32.dll")
	user32                   = windows.NewLazySystemDLL("user32.dll")
	
	procGetConsoleWindow     = kernel32.NewProc("GetConsoleWindow")
	procAllocConsole         = kernel32.NewProc("AllocConsole")
	procShowWindow          = user32.NewProc("ShowWindow")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procGetWindowLong       = user32.NewProc("GetWindowLongW")
	procSetConsoleCtrlHandler = kernel32.NewProc("SetConsoleCtrlHandler")
)

func AllocConsole() bool {
	ret, _, _ := procAllocConsole.Call()
	return ret != 0
}

func GetConsoleWindow() windows.Handle {
	ret, _, _ := procGetConsoleWindow.Call()
	return windows.Handle(ret)
}

func ShowWindow(hwnd windows.Handle, nCmdShow int) bool {
	ret, _, _ := procShowWindow.Call(
		uintptr(hwnd),
		uintptr(nCmdShow))
	return ret != 0
}

func SetForegroundWindow(hwnd windows.Handle) bool {
	ret, _, _ := procSetForegroundWindow.Call(uintptr(hwnd))
	return ret != 0
}

func GetWindowLong(hwnd windows.Handle, index int) int32 {
	ret, _, _ := procGetWindowLong.Call(
		uintptr(hwnd),
		uintptr(index))
	return int32(ret)
}

func SetConsoleCtrlHandler(handler uintptr, add bool) bool {
	ret, _, _ := procSetConsoleCtrlHandler.Call(handler, boolToUintptr(add))
	return ret != 0
}

func boolToUintptr(b bool) uintptr {
	if b {
		return 1
	}
	return 0
}

func init() {
	// 设置控制台控制处理程序
	SetConsoleCtrlHandler(0, true)
	// 设置应用程序不显示控制台窗口
	hideConsoleOnStartup()
}

func hideConsoleOnStartup() {
	console := GetConsoleWindow()
	if console != 0 {
		ShowWindow(console, SW_HIDE)
	}
} 