package main

import (
	"bytes"
	"fmt"
	"github.com/getlantern/systray"
	"golang.org/x/sys/windows"
	"io"
	"log"
	"os"
	"os/exec"
	"runtime"
	_ "embed"
)

var (
	consoleWindow windows.Handle // 保存控制台窗口句柄
	logBuffer     bytes.Buffer  // 用于缓存日志
)

//go:embed icon.ico
var iconBytes []byte

func init() {
	// 设置日志同时输出到文件和缓存
	logFile, err := os.OpenFile("deeplx.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err == nil {
		// 确保同时写入到文件和缓存
		multiWriter := io.MultiWriter(logFile, &logBuffer)
		log.SetOutput(multiWriter)
	}
}

func initTray() error {
	// 启动托盘
	go systray.Run(onReady, onExit)
	return nil
}

func onReady() {
	// 隐藏控制台窗口
	hideConsole()
	
	// 设置托盘图标（尝试多个位置和方式加载）
	iconData, err := os.ReadFile("icon.ico")
	if err != nil {
		// 使用嵌入的图标数据
		iconData = iconBytes
		log.Println("使用嵌入的图标数据")
	}
	
	if iconData != nil {
		systray.SetIcon(iconData)
		log.Println("成功加载托盘图标")
	} else {
		log.Println("警告: 未能加载托盘图标")
	}
	
	systray.SetTitle("DeepLX Local")
	systray.SetTooltip("DeepLX Local - 运行中")

	// 添加菜单项
	mShowConsole := systray.AddMenuItem("显示日志", "显示程序日志窗口")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("退出", "退出程序")

	// 处理菜单点击事件
	go func() {
		for {
			select {
			case <-mShowConsole.ClickedCh:
				if runtime.GOOS == "windows" {
					log.Println("尝试显示控制台窗口...")
					// 如果没有控制台，先分配一个
					if GetConsoleWindow() == 0 {
						if !AllocConsole() {
							log.Println("分配控制台失败")
							return
						}
						// 重定向标准输出和标准错误到新控制台
						file, err := os.OpenFile("CONOUT$", os.O_RDWR, 0)
						if err == nil {
							os.Stdout = file
							os.Stderr = file
							// 重新设置日志输出
							log.SetOutput(io.MultiWriter(file, &logBuffer))
						}
					}
					
					console := GetConsoleWindow()
					if console != 0 {
						consoleWindow = console
						log.Printf("获取到控制台窗口句柄: %v\n", console)
						ShowWindow(console, SW_SHOW)
						SetForegroundWindow(console)
						
						// 输出缓存的日志
						fmt.Print(logBuffer.String())
					} else {
						log.Println("错误: 无法获取控制台窗口句柄")
					}
				}
			case <-mQuit.ClickedCh:
				log.Println("正在退出程序...")
				systray.Quit()
				return
			}
		}
	}()
}

func onExit() {
	os.Exit(0)
}

// openBrowser 打开浏览器
func openBrowser(url string) {
	var err error
	switch os.Getenv("GOOS") {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("cmd", "/c", "start", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = exec.Command("cmd", "/c", "start", url).Start()
	}
	if err != nil {
		log.Printf("打开浏览器失败: %v\n", err)
	}
}

// 添加新的控制台相关函数
func hideConsole() {
	if runtime.GOOS == "windows" {
		console := GetConsoleWindow()
		if console != 0 {
			log.Printf("隐藏控制台窗口，句柄: %v\n", console)
			consoleWindow = console // 保存窗口句柄
			
			// 在隐藏之前确保日志输出被正确重定向
			if logFile, err := os.OpenFile("deeplx.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666); err == nil {
				multiWriter := io.MultiWriter(logFile, &logBuffer)
				log.SetOutput(multiWriter)
			}
			
			ShowWindow(console, SW_HIDE)
		} else {
			log.Println("警告: 获取控制台窗口句柄失败")
		}
	}
}

func showConsole() {
	if runtime.GOOS == "windows" {
		if consoleWindow == 0 {
			log.Println("错误: 无效的控制台窗口句柄")
			return
		}
		log.Printf("显示控制台窗口，句柄: %v\n", consoleWindow)
		if !ShowWindow(consoleWindow, SW_SHOW) {
			log.Println("ShowWindow 调用失败")
		}
		if !SetForegroundWindow(consoleWindow) {
			log.Println("SetForegroundWindow 调用失败")
		}
	}
}

func toggleConsole() {
	if runtime.GOOS == "windows" && consoleWindow != 0 {
		if isConsoleVisible() {
			ShowWindow(consoleWindow, SW_HIDE)
		} else {
			ShowWindow(consoleWindow, SW_SHOW)
			SetForegroundWindow(consoleWindow)
		}
	}
}

func isConsoleVisible() bool {
	if runtime.GOOS == "windows" && consoleWindow != 0 {
		ret := GetWindowLong(consoleWindow, GWL_STYLE)
		return (ret & WS_VISIBLE) != 0
	}
	return false
} 