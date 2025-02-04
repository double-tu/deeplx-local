//go:build windows
package main

//go:generate go install github.com/akavel/rsrc@latest
//go:generate rsrc -manifest manifest.manifest -ico icon.ico -o rsrc_windows.syso

func init() {
    // 这个文件只用于生成资源文件
} 