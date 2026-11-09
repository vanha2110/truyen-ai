package main

import (
	"context"
	"embed"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"

	"convertfix/internal/native"
	"convertfix/internal/service"
	"convertfix/internal/store"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	dir := store.DefaultDir()
	s, err := store.Open(dir)
	if err != nil {
		native.ErrorDialog(err.Error())
		return
	}
	defer s.Close()
	logs := filepath.Join(dir, "logs")
	if err = os.MkdirAll(logs, 0700); err != nil {
		println(err.Error())
		return
	}
	path := filepath.Join(logs, "convertfix.log")
	if info, e := os.Stat(path); e == nil && info.Size() > 5<<20 {
		_ = os.Remove(path + ".1")
		_ = os.Rename(path, path+".1")
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		println(err.Error())
		return
	}
	defer file.Close()
	slog.SetDefault(slog.New(slog.NewJSONHandler(file, nil)))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var ui context.Context
	var mu sync.RWMutex
	var quitFromTray atomic.Bool
	app, err := service.New(ctx, s, func(event string, data interface{}) {
		mu.RLock()
		current := ui
		mu.RUnlock()
		if current != nil {
			runtime.EventsEmit(current, event, data)
		}
	})
	if err != nil {
		slog.Error("startup", "error", err)
		return
	}
	err = wails.Run(&options.App{Title: "ConvertFix", Width: 1280, Height: 850, MinWidth: 900, MinHeight: 650, BackgroundColour: &options.RGBA{R: 247, G: 248, B: 250, A: 255}, AssetServer: &assetserver.Options{Assets: assets}, Bind: []interface{}{app},
		Windows: &windows.Options{WebviewIsTransparent: false, WindowIsTranslucent: false},
		OnStartup: func(c context.Context) {
			mu.Lock()
			ui = c
			mu.Unlock()
			native.StartTray(ctx, func() { runtime.WindowShow(c); runtime.WindowUnminimise(c) }, func() { quitFromTray.Store(true); runtime.Quit(c) })
			if slices.Contains(os.Args, "--background") {
				runtime.WindowHide(c)
			}
			if err := app.Queue.Start(ctx); err != nil {
				slog.Error("queue start", "error", err)
				_, _ = runtime.MessageDialog(c, runtime.MessageDialogOptions{Type: runtime.ErrorDialog, Title: "Không mở được hàng đợi", Message: err.Error()})
			}
		},
		OnBeforeClose: func(c context.Context) bool {
			if quitFromTray.Load() {
				return false
			}
			if !app.Queue.Active() {
				return false
			}
			answer, err := runtime.MessageDialog(c, runtime.MessageDialogOptions{Type: runtime.QuestionDialog, Title: "Đang biên tập truyện", Message: "Thoát sẽ lưu tiến độ và tiếp tục khi mở lại. Thu xuống khay để giữ app chạy nền.", Buttons: []string{"Thoát và lưu tiến độ", "Thu xuống khay", "Tiếp tục"}, DefaultButton: "Tiếp tục", CancelButton: "Tiếp tục"})
			if err != nil {
				return true
			}
			if answer == "Thu xuống khay" {
				runtime.WindowHide(c)
			}
			return answer != "Thoát và lưu tiến độ"
		},
		OnShutdown: func(context.Context) { cancel(); app.Queue.Wait(); native.StopTray() },
	})
	cancel()
	app.Queue.Wait()
	if err != nil {
		slog.Error("desktop", "error", err)
		println(err.Error())
	}
}
