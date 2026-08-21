//go:build windows
// +build windows

package main

import (
	"fmt"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"github.com/ashtray01/printerone/legacy/xp/config"
	"github.com/ashtray01/printerone/legacy/xp/spooler"
)

const (
	wmCreate          = 0x0001
	wmDestroy         = 0x0002
	wmSize            = 0x0005
	wmClose           = 0x0010
	wmQueryEndSession = 0x0011
	wmEndSession      = 0x0016
	wmCommand         = 0x0111
	wmSetFont         = 0x0030
	wmLButtonUp       = 0x0202
	wmLButtonDblClk   = 0x0203
	wmRButtonUp       = 0x0205
	wmAppLog          = 0x8001
	wmAppTray         = 0x8002
	// Fixed-size window: resizing/maximising does not improve the compact form.
	wsCompactWindow = 0x00CA0000
	wsVisible       = 0x10000000
	wsChild         = 0x40000000
	wsTabStop       = 0x00010000
	wsVScroll       = 0x00200000
	wsBorder        = 0x00800000
	bsPushButton    = 0
	bsAutoCheckBox  = 3
	esAutoVScroll   = 0x0040
	esMultiLine     = 0x0004
	esReadOnly      = 0x0800
	cbsDropDownList = 0x0003
	swHide          = 0
	swShow          = 5
	sizeMinimized   = 1
	colorBtnFace    = 15
	defaultGuiFont  = 17
	cbAddString     = 0x0143
	cbResetContent  = 0x014b
	cbGetCurSel     = 0x0147
	cbGetLBText     = 0x0148
	cbSetCurSel     = 0x014e
	bmGetCheck      = 0x00f0
	bmSetCheck      = 0x00f1
	bstChecked      = 1
	emSetSel        = 0x00b1
	emReplaceSel    = 0x00c2
	nimAdd          = 0
	nimDelete       = 2
	nifMessage      = 1
	nifIcon         = 2
	nifTip          = 4
	idiApplication  = 32512
	idcArrow        = 32512

	idPrinter       = 101
	idRefresh       = 102
	idAddress       = 103
	idPort          = 104
	idAutoStart     = 105
	idWindowsStart  = 106
	idMinimize      = 107
	idSave          = 108
	idStart         = 109
	idStop          = 110
	idCheck         = 111
	idTest          = 112
	idExit          = 113
	idLanguage      = 114
	idApplyLanguage = 115
)

var (
	user32                  = syscall.NewLazyDLL("user32.dll")
	kernel32ui              = syscall.NewLazyDLL("kernel32.dll")
	shell32                 = syscall.NewLazyDLL("shell32.dll")
	gdi32                   = syscall.NewLazyDLL("gdi32.dll")
	procRegisterClassEx     = user32.NewProc("RegisterClassExW")
	procCreateWindowEx      = user32.NewProc("CreateWindowExW")
	procDefWindowProc       = user32.NewProc("DefWindowProcW")
	procShowWindow          = user32.NewProc("ShowWindow")
	procUpdateWindow        = user32.NewProc("UpdateWindow")
	procGetMessage          = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessage     = user32.NewProc("DispatchMessageW")
	procPostQuitMessage     = user32.NewProc("PostQuitMessage")
	procDestroyWindow       = user32.NewProc("DestroyWindow")
	procSetWindowText       = user32.NewProc("SetWindowTextW")
	procEnableWindow        = user32.NewProc("EnableWindow")
	procGetWindowText       = user32.NewProc("GetWindowTextW")
	procSendMessage         = user32.NewProc("SendMessageW")
	procPostMessage         = user32.NewProc("PostMessageW")
	procMessageBox          = user32.NewProc("MessageBoxW")
	procLoadCursor          = user32.NewProc("LoadCursorW")
	procLoadIcon            = user32.NewProc("LoadIconW")
	procGetModuleHandle     = kernel32ui.NewProc("GetModuleHandleW")
	procGetStockObject      = gdi32.NewProc("GetStockObject")
	procShellNotifyIcon     = shell32.NewProc("Shell_NotifyIconW")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	windowCallback          = syscall.NewCallback(windowProc)
)

type point struct{ X, Y int32 }
type message struct {
	Hwnd           uintptr
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	Pt             point
}
type windowClassEx struct {
	Size, Style                                                        uint32
	WndProc                                                            uintptr
	ClsExtra, WndExtra                                                 int32
	Instance, Icon, Cursor, Background, MenuName, ClassName, IconSmall uintptr
}
type notifyIconData struct {
	Size                       uint32
	Window                     uintptr
	ID, Flags, CallbackMessage uint32
	Icon                       uintptr
	Tip                        [128]uint16
	State, StateMask           uint32
	Info                       [256]uint16
	TimeoutOrVersion           uint32
	InfoTitle                  [64]uint16
	InfoFlags                  uint32
}
type controls struct {
	printer, address, port, language, languageApply, autoStart, windowsStart, minimize, status, log uintptr
	start, stop, footer, printerCount                                                               uintptr
}

func (a *application) runWindow() error {
	instance, _, _ := procGetModuleHandle.Call(0)
	text := a.text()
	className := utf16("PrinterOneXPWindow")
	icon := loadAppIcon()
	cursor, _, _ := procLoadCursor.Call(0, idcArrow)
	wc := windowClassEx{Size: uint32(unsafe.Sizeof(windowClassEx{})), WndProc: windowCallback, Instance: instance, Icon: icon, Cursor: cursor, Background: colorBtnFace + 1, ClassName: uintptr(unsafe.Pointer(className)), IconSmall: icon}
	if result, _, callErr := procRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc))); result == 0 {
		return fmt.Errorf("register window class: %v", callErr)
	}
	title := utf16(text.Title)
	hwnd, _, callErr := procCreateWindowEx.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(title)), wsCompactWindow|wsVisible, 100, 80, 390, 420, 0, 0, instance, 0)
	if hwnd == 0 {
		return fmt.Errorf("create window: %v", callErr)
	}
	a.hwnd = hwnd
	a.addTrayIcon()
	a.loadControls()
	a.refreshPrinters()
	for _, line := range a.drainLogs() {
		a.appendLogControl(line)
	}
	procShowWindow.Call(hwnd, swShow)
	procUpdateWindow.Call(hwnd)
	if a.currentConfig().AutoStart && a.currentConfig().PrinterName != "" {
		go func() { fatalIf(a.server.Start()); postMessage(a.hwnd, wmAppLog, 0, 0) }()
	}
	a.updateStatus()
	var msg message
	for {
		result, _, err := procGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(result) == -1 {
			return err
		}
		if result == 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
	}
	return nil
}

func windowProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	if app == nil {
		result, _, _ := procDefWindowProc.Call(hwnd, uintptr(msg), wParam, lParam)
		return result
	}
	switch msg {
	case wmCreate:
		app.hwnd = hwnd
		app.createControls(hwnd)
		return 0
	case wmCommand:
		app.command(int(uint16(wParam & 0xffff)))
		return 0
	case wmAppLog:
		for _, line := range app.drainLogs() {
			app.appendLogControl(line)
		}
		app.updateStatus()
		return 0
	case wmAppTray:
		if uint32(lParam) == wmLButtonUp || uint32(lParam) == wmLButtonDblClk || uint32(lParam) == wmRButtonUp {
			procShowWindow.Call(hwnd, swShow)
			procSetForegroundWindow.Call(hwnd)
		}
		return 0
	case wmSize:
		if wParam == sizeMinimized && app.shouldMinimizeToTray() {
			procShowWindow.Call(hwnd, swHide)
		}
		return 0
	case wmClose:
		if app.shouldMinimizeToTray() && app.trayAdded && !app.exiting {
			procShowWindow.Call(hwnd, swHide)
			return 0
		}
		procDestroyWindow.Call(hwnd)
		return 0
	case wmQueryEndSession:
		return 1
	case wmEndSession:
		if wParam != 0 {
			app.exiting = true
			if app.server != nil {
				app.server.Stop()
			}
			_ = app.clearStaleJobs()
			procDestroyWindow.Call(hwnd)
		}
		return 0
	case wmDestroy:
		app.deleteTrayIcon()
		if app.server != nil {
			app.server.Stop()
		}
		if err := app.clearStaleJobs(); err != nil {
			app.addLog("[WARN] Could not remove stale print jobs: " + err.Error())
		}
		procPostQuitMessage.Call(0)
		return 0
	}
	result, _, _ := procDefWindowProc.Call(hwnd, uintptr(msg), wParam, lParam)
	return result
}

func (a *application) createControls(hwnd uintptr) {
	text := a.text()
	label(hwnd, text.Printer, 8, 14, 53, 18)
	a.controls.printer = control(hwnd, "COMBOBOX", "", cbsDropDownList|wsVScroll|wsTabStop, 63, 10, 218, 200, idPrinter)
	control(hwnd, "BUTTON", text.Refresh, bsPushButton|wsTabStop, 288, 9, 85, 25, idRefresh)

	label(hwnd, text.Address, 8, 44, 53, 18)
	a.controls.address = control(hwnd, "EDIT", "", wsBorder|wsTabStop, 63, 40, 105, 22, idAddress)
	label(hwnd, text.Port, 180, 44, 38, 18)
	a.controls.port = control(hwnd, "EDIT", "", wsBorder|wsTabStop, 220, 40, 48, 22, idPort)
	a.controls.language = control(hwnd, "COMBOBOX", "", cbsDropDownList|wsVScroll|wsTabStop, 274, 40, 70, 160, idLanguage)
	a.controls.languageApply = control(hwnd, "BUTTON", "OK", bsPushButton|wsTabStop, 348, 40, 25, 22, idApplyLanguage)

	a.controls.autoStart = control(hwnd, "BUTTON", text.AutoStart, bsAutoCheckBox|wsTabStop, 8, 68, 100, 21, idAutoStart)
	a.controls.windowsStart = control(hwnd, "BUTTON", text.WindowsStart, bsAutoCheckBox|wsTabStop, 125, 68, 105, 21, idWindowsStart)
	a.controls.minimize = control(hwnd, "BUTTON", text.Minimize, bsAutoCheckBox|wsTabStop, 245, 68, 125, 21, idMinimize)

	a.controls.status = label(hwnd, text.Stopped, 8, 94, 365, 18)

	control(hwnd, "BUTTON", text.Save, bsPushButton|wsTabStop, 8, 118, 112, 26, idSave)
	a.controls.start = control(hwnd, "BUTTON", text.Start, bsPushButton|wsTabStop, 130, 118, 112, 26, idStart)
	a.controls.stop = control(hwnd, "BUTTON", text.Stop, bsPushButton|wsTabStop, 252, 118, 112, 26, idStop)
	control(hwnd, "BUTTON", text.CheckPort, bsPushButton|wsTabStop, 8, 150, 112, 26, idCheck)
	control(hwnd, "BUTTON", text.TestPrint, bsPushButton|wsTabStop, 130, 150, 112, 26, idTest)
	control(hwnd, "BUTTON", text.Exit, bsPushButton|wsTabStop, 252, 150, 112, 26, idExit)

	label(hwnd, text.Log, 8, 182, 90, 17)
	a.controls.log = control(hwnd, "EDIT", "", wsBorder|wsVScroll|esAutoVScroll|esMultiLine|esReadOnly, 8, 200, 356, 145, 120)
	a.controls.footer = control(hwnd, "STATIC", "  "+text.Ready, wsBorder, 8, 353, 225, 21, 121)
	a.controls.printerCount = control(hwnd, "STATIC", "  "+fmt.Sprintf(text.Printers, 0), wsBorder, 240, 353, 124, 21, 122)
}

func (a *application) command(id int) {
	switch id {
	case idRefresh:
		a.refreshPrinters()
	case idSave:
		fatalIf(a.saveFromWindow())
	case idApplyLanguage:
		fatalIf(a.applyLanguage())
	case idStart:
		fatalIf(a.startServer())
	case idStop:
		a.stopServer()
	case idCheck:
		go func() { fatalIf(a.testConnection(false)) }()
	case idTest:
		go func() { fatalIf(a.testConnection(true)) }()
	case idExit:
		a.exiting = true
		procDestroyWindow.Call(a.hwnd)
	}
}

func (a *application) shouldMinimizeToTray() bool {
	if a.controls.minimize != 0 {
		return checked(a.controls.minimize)
	}
	return a.currentConfig().MinimizeToTray
}

func (a *application) loadControls() {
	cfg := a.currentConfig()
	setText(a.controls.address, cfg.ListenAddress)
	setText(a.controls.port, strconv.Itoa(cfg.Port))
	setChecked(a.controls.autoStart, cfg.AutoStart)
	setChecked(a.controls.windowsStart, cfg.StartWithWindows)
	setChecked(a.controls.minimize, cfg.MinimizeToTray)
	for index, option := range languageOptions {
		value := utf16(option.name)
		procSendMessage.Call(a.controls.language, cbAddString, 0, uintptr(unsafe.Pointer(value)))
		if option.code == cfg.Language {
			procSendMessage.Call(a.controls.language, cbSetCurSel, uintptr(index), 0)
		}
	}
}

func (a *application) readConfigFromControls() (config.Config, error) {
	cfg := a.currentConfig()
	cfg.PrinterName = comboText(a.controls.printer)
	cfg.ListenAddress = strings.TrimSpace(getText(a.controls.address))
	port, err := strconv.Atoi(strings.TrimSpace(getText(a.controls.port)))
	if err != nil {
		return cfg, fmt.Errorf("invalid port")
	}
	cfg.Port = port
	cfg.AutoStart = checked(a.controls.autoStart)
	cfg.StartWithWindows = checked(a.controls.windowsStart)
	cfg.MinimizeToTray = checked(a.controls.minimize)
	languageIndex := comboIndex(a.controls.language)
	if languageIndex >= 0 && languageIndex < len(languageOptions) {
		cfg.Language = languageOptions[languageIndex].code
	}
	cfg.LANEnabled = true
	cfg.SharedToken = ""
	return cfg, cfg.Validate()
}

func (a *application) refreshPrinters() {
	current := a.currentConfig().PrinterName
	items, err := spooler.List()
	if err != nil {
		fatalIf(fmt.Errorf("list printers: %v", err))
		return
	}
	procSendMessage.Call(a.controls.printer, cbResetContent, 0, 0)
	for _, item := range items {
		value := utf16(item)
		index, _, _ := procSendMessage.Call(a.controls.printer, cbAddString, 0, uintptr(unsafe.Pointer(value)))
		if item == current {
			procSendMessage.Call(a.controls.printer, cbSetCurSel, index, 0)
		}
	}
	if current == "" && len(items) > 0 {
		procSendMessage.Call(a.controls.printer, cbSetCurSel, 0, 0)
	}
	setText(a.controls.printerCount, "  "+fmt.Sprintf(a.text().Printers, len(items)))
	a.addLog(fmt.Sprintf("[INFO] Printers found: %d", len(items)))
}

func (a *application) updateStatus() {
	if a.controls.status == 0 {
		return
	}
	cfg := a.currentConfig()
	text := a.text()
	if a.server != nil && a.server.Running() {
		setText(a.controls.status, fmt.Sprintf("%s: %s:%d", text.Running, cfg.ListenAddress, cfg.Port))
		setText(a.controls.footer, "  "+text.ServerStarted)
		setText(a.controls.start, text.Started)
		setEnabled(a.controls.start, false)
		setEnabled(a.controls.stop, true)
	} else {
		setText(a.controls.status, text.Stopped)
		setText(a.controls.footer, "  "+text.Ready)
		setText(a.controls.start, text.Start)
		setEnabled(a.controls.start, true)
		setEnabled(a.controls.stop, false)
	}
}

func (a *application) appendLogControl(line string) {
	if a.controls.log == 0 {
		return
	}
	procSendMessage.Call(a.controls.log, emSetSel, ^uintptr(0), ^uintptr(0))
	value := utf16(line + "\r\n")
	procSendMessage.Call(a.controls.log, emReplaceSel, 0, uintptr(unsafe.Pointer(value)))
}

func (a *application) addTrayIcon() {
	icon := loadAppIcon()
	data := notifyIconData{Size: uint32(unsafe.Sizeof(notifyIconData{})), Window: a.hwnd, ID: 1, Flags: nifMessage | nifIcon | nifTip, CallbackMessage: wmAppTray, Icon: icon}
	copy(data.Tip[:], syscall.StringToUTF16("PrinterOne XP"))
	ok, _, _ := procShellNotifyIcon.Call(nimAdd, uintptr(unsafe.Pointer(&data)))
	a.trayAdded = ok != 0
}

func (a *application) deleteTrayIcon() {
	if !a.trayAdded {
		return
	}
	data := notifyIconData{Size: uint32(unsafe.Sizeof(notifyIconData{})), Window: a.hwnd, ID: 1}
	procShellNotifyIcon.Call(nimDelete, uintptr(unsafe.Pointer(&data)))
	a.trayAdded = false
}

func control(parent uintptr, class, text string, style uint32, x, y, width, height, id int) uintptr {
	classPtr, textPtr := utf16(class), utf16(text)
	hwnd, _, _ := procCreateWindowEx.Call(0, uintptr(unsafe.Pointer(classPtr)), uintptr(unsafe.Pointer(textPtr)), uintptr(wsChild|wsVisible|style), uintptr(x), uintptr(y), uintptr(width), uintptr(height), parent, uintptr(id), 0, 0)
	font, _, _ := procGetStockObject.Call(defaultGuiFont)
	procSendMessage.Call(hwnd, wmSetFont, font, 1)
	return hwnd
}
func label(parent uintptr, text string, x, y, width, height int) uintptr {
	return control(parent, "STATIC", text, 0, x, y, width, height, 0)
}
func utf16(value string) *uint16 { result, _ := syscall.UTF16PtrFromString(value); return result }
func setText(hwnd uintptr, value string) {
	ptr := utf16(value)
	procSetWindowText.Call(hwnd, uintptr(unsafe.Pointer(ptr)))
}
func getText(hwnd uintptr) string {
	buf := make([]uint16, 1024)
	procGetWindowText.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return syscall.UTF16ToString(buf)
}
func setChecked(hwnd uintptr, value bool) {
	state := uintptr(0)
	if value {
		state = bstChecked
	}
	procSendMessage.Call(hwnd, bmSetCheck, state, 0)
}
func setEnabled(hwnd uintptr, value bool) {
	enabled := uintptr(0)
	if value {
		enabled = 1
	}
	procEnableWindow.Call(hwnd, enabled)
}
func loadAppIcon() uintptr {
	instance, _, _ := procGetModuleHandle.Call(0)
	icon, _, _ := procLoadIcon.Call(instance, 1)
	if icon == 0 {
		icon, _, _ = procLoadIcon.Call(0, idiApplication)
	}
	return icon
}
func checked(hwnd uintptr) bool {
	state, _, _ := procSendMessage.Call(hwnd, bmGetCheck, 0, 0)
	return state == bstChecked
}
func comboText(hwnd uintptr) string {
	index, _, _ := procSendMessage.Call(hwnd, cbGetCurSel, 0, 0)
	if int32(index) < 0 {
		return ""
	}
	buf := make([]uint16, 1024)
	procSendMessage.Call(hwnd, cbGetLBText, index, uintptr(unsafe.Pointer(&buf[0])))
	return syscall.UTF16ToString(buf)
}
func comboIndex(hwnd uintptr) int {
	index, _, _ := procSendMessage.Call(hwnd, cbGetCurSel, 0, 0)
	return int(int32(index))
}
func postMessage(hwnd uintptr, msg uint32, wParam, lParam uintptr) {
	procPostMessage.Call(hwnd, uintptr(msg), wParam, lParam)
}
func messageBox(hwnd uintptr, text, title string) {
	textPtr, titlePtr := utf16(text), utf16(title)
	procMessageBox.Call(hwnd, uintptr(unsafe.Pointer(textPtr)), uintptr(unsafe.Pointer(titlePtr)), 0x10)
}
