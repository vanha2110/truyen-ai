package settings

import (
	"convertfix/internal/domain"
	"convertfix/internal/ollama"
	"errors"
	"time"
)

func Validate(c domain.Config) error {
	if c.Prefetch < 0 || c.Prefetch > 10 {
		return errors.New("Số chương biên tập trước cần từ 0 đến 10")
	}
	if _, err := ollama.New(c.OllamaURL); err != nil {
		return err
	}
	if c.Temperature < 0 || c.Temperature > 2 || c.TopP <= 0 || c.TopP > 1 || c.RepeatPenalty < 1 || c.RepeatPenalty > 2 {
		return errors.New("Tham số sampling không hợp lệ")
	}
	if c.NumCtx < 2048 || c.NumCtx > 1048576 || c.NumPredict < 128 || c.NumPredict >= c.NumCtx {
		return errors.New("Context cần từ 2048 đến 1048576; output phải nhỏ hơn context")
	}
	if c.ChunkWords < 50 || c.ChunkWords > 2000 || c.Retries < 0 || c.Retries > 3 {
		return errors.New("Chunk cần 50–2000 từ; số lần retry cần 0–3")
	}
	if _, err := time.ParseDuration(c.KeepAlive); err != nil {
		return errors.New("Keep alive cần dạng 10m hoặc 1h")
	}
	if c.NightStart < 0 || c.NightStart > 23 || c.NightEnd < 0 || c.NightEnd > 23 || c.NightStart == c.NightEnd {
		return errors.New("Giờ chạy đêm không hợp lệ")
	}
	return nil
}
func InSchedule(c domain.Config, now time.Time) bool {
	if !c.NightOnly {
		return true
	}
	h := now.Hour()
	if c.NightStart < c.NightEnd {
		return h >= c.NightStart && h < c.NightEnd
	}
	return h >= c.NightStart || h < c.NightEnd
}
