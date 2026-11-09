package service

import (
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"runtime"
	"strings"

	"convertfix/internal/domain"
	"convertfix/internal/updater"
)

func (a *App) ImportConfig(raw string) error {
	c := domain.DefaultConfig()
	d := json.NewDecoder(strings.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&c); err != nil {
		return err
	}
	if d.Decode(new(interface{})) != io.EOF {
		return errors.New("JSON chứa dữ liệu thừa")
	}
	previous, err := a.Store.Config()
	if err != nil {
		return err
	}
	c.Autostart = previous.Autostart
	return a.SaveConfig(c)
}
func (a *App) Diagnostics() (string, error) {
	var schema string
	err := a.Store.DB.QueryRow(`SELECT COALESCE(max(version),'') FROM schema_version`).Scan(&schema)
	if err != nil {
		return "", err
	}
	var integrity string
	if err = a.Store.DB.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil {
		return "", err
	}
	jobs, err := a.Store.Jobs()
	if err != nil {
		return "", err
	}
	counts := map[domain.Status]int{}
	for _, j := range jobs {
		counts[j.Status]++
	}
	report := struct {
		Version   string                `json:"version"`
		OS        string                `json:"os"`
		Arch      string                `json:"arch"`
		Go        string                `json:"go"`
		Schema    string                `json:"schema"`
		Integrity string                `json:"integrity"`
		Jobs      map[domain.Status]int `json:"jobs"`
	}{updater.Version, runtime.GOOS, runtime.GOARCH, runtime.Version(), schema, integrity, counts}
	b, err := json.MarshalIndent(report, "", "  ")
	return string(b), err
}
func (a *App) UpdateChannel() (updater.Channel, error) {
	var c updater.Channel
	err := a.Store.GetJSON("update_channel", &c)
	return c, err
}
func (a *App) SaveUpdateChannel(c updater.Channel) error {
	if err := updater.ValidateChannel(c); err != nil {
		return err
	}
	return a.Store.PutJSON("update_channel", c)
}
func (a *App) CheckUpdate() (updater.Release, error) {
	c, err := a.UpdateChannel()
	if err != nil {
		return updater.Release{}, err
	}
	return updater.Check(a.ctx, c)
}
func (a *App) DownloadUpdate() (string, error) {
	c, err := a.UpdateChannel()
	if err != nil {
		return "", err
	}
	if _, err = a.Store.Backup(); err != nil {
		return "", err
	}
	return updater.Download(a.ctx, c, filepath.Join(a.Store.Dir, "updates"))
}
