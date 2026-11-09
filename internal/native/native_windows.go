package native

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"runtime"
	"sync"
	"unsafe"

	"git.sr.ht/~jackmordaunt/go-toast/v2"
	"github.com/getlantern/systray"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

//go:embed icon.ico
var icon []byte
var stopOnce sync.Once

func StartTray(ctx context.Context, show, quit func()) {
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		systray.Run(func() {
			systray.SetIcon(icon)
			systray.SetTooltip("ConvertFix — đang chạy trên máy bạn")
			open := systray.AddMenuItem("Mở ConvertFix", "Mở cửa sổ đọc truyện")
			systray.AddSeparator()
			exit := systray.AddMenuItem("Thoát và lưu tiến độ", "Dừng worker an toàn")
			go func() {
				for {
					select {
					case <-ctx.Done():
						StopTray()
						return
					case <-open.ClickedCh:
						show()
					case <-exit.ClickedCh:
						quit()
					}
				}
			}()
		}, func() {})
	}()
}
func StopTray() { stopOnce.Do(systray.Quit) }
func Autostart(enable bool) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	if !enable {
		err = key.DeleteValue("ConvertFix")
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return key.SetStringValue("ConvertFix", fmt.Sprintf("%q --background", exe))
}

var toastOnce sync.Once
var toastErr error

func Notify(title, message string) error {
	toastOnce.Do(func() {
		toastErr = toast.SetAppData(toast.AppData{AppID: "ConvertFix", GUID: "{48AC1202-E22A-4AD0-88CA-0CD732E78EE1}"})
	})
	if toastErr != nil {
		return toastErr
	}
	n := toast.Notification{AppID: "ConvertFix", Title: title, Body: message}
	return n.Push()
}
func ErrorDialog(message string) {
	title, _ := windows.UTF16PtrFromString("ConvertFix")
	body, _ := windows.UTF16PtrFromString(message)
	proc := windows.NewLazySystemDLL("user32.dll").NewProc("MessageBoxW")
	_, _, _ = proc.Call(0, uintptr(unsafe.Pointer(body)), uintptr(unsafe.Pointer(title)), 0x10)
}
