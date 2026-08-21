package main

import (
	"embed"
	"errors"
	"os"

	"github.com/ashtray01/printerone/internal/instance"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"golang.org/x/sys/windows"
)

//go:embed all:frontend/dist build/windows/icon.ico
var assets embed.FS

//go:embed build/windows/icon.ico
var appIcon []byte

func main() {
	guard, err := instance.Acquire()
	if errors.Is(err, instance.ErrAlreadyRunning) {
		showAlreadyRunning()
		return
	}
	if err != nil {
		println("Unable to start PrinterOne:", err.Error())
		return
	}
	defer guard.Close()

	// Create an instance of the app structure
	app := NewApp()

	// Create application with options
	err = wails.Run(&options.App{
		Title:     "PrinterOne — Network Print Server",
		Width:     1080,
		Height:    720,
		MinWidth:  900,
		MinHeight: 620,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		OnBeforeClose:    app.beforeClose,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

func showAlreadyRunning() {
	text, _ := windows.UTF16PtrFromString("Приложение уже запущено.")
	title, _ := windows.UTF16PtrFromString("PrinterOne")
	windows.MessageBox(0, text, title, windows.MB_OK|windows.MB_ICONINFORMATION)
	_, _ = os.Stdout.WriteString("Приложение уже запущено.\n")
}
