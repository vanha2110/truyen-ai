//go:build !windows

package native

import (
	"context"
	"errors"
)

func StartTray(context.Context, func(), func()) {}
func StopTray()                                 {}
func Autostart(bool) error {
	return errors.New("Khởi động cùng hệ thống chỉ hỗ trợ Windows")
}
func Notify(string, string) error { return nil }
func ErrorDialog(message string)  { println(message) }
