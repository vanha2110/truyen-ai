package settings

import (
	"convertfix/internal/domain"
	"testing"
	"time"
)

func TestNightScheduleAcrossMidnight(t *testing.T) {
	c := domain.DefaultConfig()
	c.NightOnly = true
	for hour := 0; hour < 24; hour++ {
		want := hour >= 22 || hour < 7
		if got := InSchedule(c, time.Date(2026, 11, 9, hour, 0, 0, 0, time.Local)); got != want {
			t.Fatalf("hour %d: %t", hour, got)
		}
	}
	c.NumPredict = c.NumCtx
	if Validate(c) == nil {
		t.Fatal("unbounded output accepted")
	}
}
