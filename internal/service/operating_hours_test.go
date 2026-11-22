package service

import (
	"testing"
	"time"
)

func TestIsWithinOperatingHours(t *testing.T) {
	// 2026-11-23 is a Monday.
	wib := time.FixedZone("Asia/Jakarta", 7*3600)
	atWIB := func(year int, month time.Month, day, hour, min int) time.Time {
		return time.Date(year, month, day, hour, min, 0, 0, wib).UTC()
	}

	const standardWeek = `{"mon":"07:00-22:00","tue":"07:00-22:00","wed":"07:00-22:00","thu":"07:00-22:00","fri":"07:00-23:00","sat":"07:00-23:00","sun":"07:00-22:00"}`

	tests := []struct {
		name     string
		nowUTC   time.Time
		timezone string
		hoursRaw []byte
		want     bool
	}{
		{
			name:     "nil hours fail-open",
			nowUTC:   atWIB(2026, time.November, 23, 2, 0),
			timezone: "Asia/Jakarta",
			hoursRaw: nil,
			want:     true,
		},
		{
			name:     "empty object fail-open",
			nowUTC:   atWIB(2026, time.November, 23, 2, 0),
			timezone: "Asia/Jakarta",
			hoursRaw: []byte(`{}`),
			want:     true,
		},
		{
			name:     "corrupt json fail-closed",
			nowUTC:   atWIB(2026, time.November, 23, 10, 0),
			timezone: "Asia/Jakarta",
			hoursRaw: []byte(`{"mon":`),
			want:     false,
		},
		{
			name:     "non-string value fail-closed",
			nowUTC:   atWIB(2026, time.November, 23, 10, 0),
			timezone: "Asia/Jakarta",
			hoursRaw: []byte(`{"mon":123}`),
			want:     false,
		},
		{
			name:     "invalid window syntax fail-closed",
			nowUTC:   atWIB(2026, time.November, 23, 10, 0),
			timezone: "Asia/Jakarta",
			hoursRaw: []byte(`{"mon":"25:00-26:00"}`),
			want:     false,
		},
		{
			name:     "before open boundary closed",
			nowUTC:   atWIB(2026, time.November, 23, 6, 59),
			timezone: "Asia/Jakarta",
			hoursRaw: []byte(standardWeek),
			want:     false,
		},
		{
			name:     "exact open boundary open",
			nowUTC:   atWIB(2026, time.November, 23, 7, 0),
			timezone: "Asia/Jakarta",
			hoursRaw: []byte(standardWeek),
			want:     true,
		},
		{
			name:     "midday open",
			nowUTC:   atWIB(2026, time.November, 23, 14, 30),
			timezone: "Asia/Jakarta",
			hoursRaw: []byte(standardWeek),
			want:     true,
		},
		{
			name:     "one minute before close open",
			nowUTC:   atWIB(2026, time.November, 23, 21, 59),
			timezone: "Asia/Jakarta",
			hoursRaw: []byte(standardWeek),
			want:     true,
		},
		{
			name:     "exact close boundary closed",
			nowUTC:   atWIB(2026, time.November, 23, 22, 0),
			timezone: "Asia/Jakarta",
			hoursRaw: []byte(standardWeek),
			want:     false,
		},
		{
			name:     "utc conversion respects Asia/Jakarta (15:30 UTC == 22:30 WIB)",
			nowUTC:   time.Date(2026, time.November, 23, 15, 30, 0, 0, time.UTC),
			timezone: "",
			hoursRaw: []byte(standardWeek),
			want:     false,
		},
		{
			name:     "closed marker on monday",
			nowUTC:   atWIB(2026, time.November, 23, 12, 0),
			timezone: "Asia/Jakarta",
			hoursRaw: []byte(`{"mon":"closed","tue":"07:00-22:00"}`),
			want:     false,
		},
		{
			name:     "missing weekday in non-empty schedule is closed",
			nowUTC:   atWIB(2026, time.November, 23, 12, 0),
			timezone: "Asia/Jakarta",
			hoursRaw: []byte(`{"tue":"07:00-22:00"}`),
			want:     false,
		},
		{
			name:     "overnight window late evening open",
			nowUTC:   atWIB(2026, time.November, 23, 23, 30),
			timezone: "Asia/Jakarta",
			hoursRaw: []byte(`{"mon":"18:00-02:00","tue":"closed"}`),
			want:     true,
		},
		{
			name:     "overnight window carries over to next morning",
			nowUTC:   atWIB(2026, time.November, 24, 1, 30),
			timezone: "Asia/Jakarta",
			hoursRaw: []byte(`{"mon":"18:00-02:00","tue":"closed"}`),
			want:     true,
		},
		{
			name:     "overnight window closes at end time next morning",
			nowUTC:   atWIB(2026, time.November, 24, 2, 0),
			timezone: "Asia/Jakarta",
			hoursRaw: []byte(`{"mon":"18:00-02:00","tue":"closed"}`),
			want:     false,
		},
		{
			name:     "24h store window open at midnight and 23:59",
			nowUTC:   atWIB(2026, time.November, 23, 23, 59),
			timezone: "Asia/Jakarta",
			hoursRaw: []byte(`{"mon":"00:00-24:00"}`),
			want:     true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isWithinOperatingHours(tc.nowUTC, tc.timezone, tc.hoursRaw)
			if got != tc.want {
				t.Fatalf("isWithinOperatingHours(%s, %q, %s) = %v, want %v",
					tc.nowUTC.Format(time.RFC3339), tc.timezone, string(tc.hoursRaw), got, tc.want)
			}
		})
	}
}
