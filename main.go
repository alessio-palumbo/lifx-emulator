package main

import (
	"embed"
	"fmt"
	"strings"

	"lifx-emulator/internal/app"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

var version = "0.1.0"

func appTitle() string {
	return fmt.Sprintf("LIFX Emulator v%s", strings.TrimPrefix(version, "v"))
}

func main() {
	a := app.New()
	if err := wails.Run(&options.App{Title: appTitle(), Width: 1180, Height: 820, MinWidth: 720, MinHeight: 500, AssetServer: &assetserver.Options{Assets: assets}, Linux: &linux.Options{Icon: appIcon}, OnStartup: a.Startup, OnShutdown: a.Shutdown, Bind: []interface{}{a}, BackgroundColour: &options.RGBA{R: 20, G: 22, B: 28, A: 1}}); err != nil {
		println(err.Error())
	}
}
