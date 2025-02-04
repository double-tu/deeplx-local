//go:build windows
package main

import (
	"golang.org/x/sys/windows"
	"syscall"
)

const (
	SW_HIDE = 0
	SW_SHOW = 5
	
	WS_VISIBLE = 0x10000000
	GWL_STYLE  = -16
	CTRL_CLOSE_EVENT = 2
	GWL_WNDPROC = -4
	WM_CLOSE    = 0x0010
)

var (
	kernel32                 = windows.NewLazySystemDLL("kernel32.dll")
	user32                   = windows.NewLazySystemDLL("user32.dll")
	
	procGetConsoleWindow     = kernel32.NewProc("GetConsoleWindow")
	procAllocConsole         = kernel32.NewProc("AllocConsole")
	procShowWindow          = user32.NewProc("ShowWindow")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procGetWindowLong       = user32.NewProc("GetWindowLongW")
	procSetWindowLong      = user32.NewProc("SetWindowLongW")
	procCallWindowProc     = user32.NewProc("CallWindowProcW")
	procSetConsoleCtrlHandler = kernel32.NewProc("SetConsoleCtrlHandler")
	
	handlerRoutine = syscall.NewCallback(func(controlType uint32) uintptr {
		switch controlType {
		case CTRL_CLOSE_EVENT:
			console := GetConsoleWindow()
			if console != 0 {
				ShowWindow(console, SW_HIDE)
			}
			return 1
		}
		return 0
	})
	
	oldWndProc uintptr
)

func AllocConsole() bool {
	ret, _, _ := procAllocConsole.Call()
	if ret != 0 {
		// 获取新分配的控制台窗口句柄
		hwnd := GetConsoleWindow()
		if hwnd != 0 {
			// 替换窗口过程
			newProc := windows.NewCallback(wndProc)
			oldWndProc = setWindowLongPtr(hwnd, GWL_WNDPROC, newProc)
		}
	}
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

func setWindowLongPtr(hwnd windows.Handle, index int, newProc uintptr) uintptr {
	ret, _, _ := procSetWindowLong.Call(
		uintptr(hwnd),
		uintptr(index),
		newProc)
	return ret
}

func callWindowProc(prevWndProc uintptr, hwnd windows.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	ret, _, _ := procCallWindowProc.Call(
		prevWndProc,
		uintptr(hwnd),
		uintptr(msg),
		wParam,
		lParam)
	return ret
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
	SetConsoleCtrlHandler(handlerRoutine, true)
	// 设置应用程序不显示控制台窗口
	hideConsoleOnStartup()
}

func hideConsoleOnStartup() {
	console := GetConsoleWindow()
	if console != 0 {
		ShowWindow(console, SW_HIDE)
	}
}

func wndProc(hwnd windows.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_CLOSE:
		// 截获关闭消息，改为隐藏窗口
		ShowWindow(hwnd, SW_HIDE)
		return 0
	}
	return callWindowProc(oldWndProc, hwnd, msg, wParam, lParam)
} 