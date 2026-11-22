package service

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

const defaultStoreTimezone = "Asia/Jakarta"

var (
	weekdayScheduleKeys = [7]string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}
	wibFixedZone        = time.FixedZone(defaultStoreTimezone, 7*3600)
)

// isWithinOperatingHours evaluates whether nowUTC falls inside the store's configured
// operating_hours schedule in its local timezone (falling back to Asia/Jakarta).
//
// Policy:
//   - Unconfigured (nil, blank, or empty JSON object "{}") returns true (fail-open).
//   - Malformed JSON or non-string/invalid day window values return false (fail-closed).
//   - Supports normal windows ("07:00-22:00"), 24h ("00:00-24:00"), and overnight windows
//     ("18:00-02:00", checked against both today's and yesterday's schedule).
func isWithinOperatingHours(nowUTC time.Time, timezone string, hoursRaw []byte) bool {
	trimmed := bytes.TrimSpace(hoursRaw)
	if len(trimmed) == 0 {
		return true
	}

	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &rawMap); err != nil || rawMap == nil {
		return false
	}
	if len(rawMap) == 0 {
		return true
	}

	schedule := make(map[string]string, len(rawMap))
	for k, v := range rawMap {
		var val string
		if err := json.Unmarshal(v, &val); err != nil {
			return false
		}
		schedule[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(val)
	}

	loc := loadStoreLocation(timezone)
	local := nowUTC.In(loc)
	curMin := local.Hour()*60 + local.Minute()
	todayKey := weekdayScheduleKeys[local.Weekday()]
	yesterdayKey := weekdayScheduleKeys[(local.Weekday()+6)%7]

	if val, ok := schedule[todayKey]; ok {
		startMin, endMin, closed, valid := parseOperatingWindow(val)
		if !valid {
			return false
		}
		if !closed {
			if startMin < endMin && curMin >= startMin && curMin < endMin {
				return true
			}
			if startMin > endMin && curMin >= startMin {
				return true
			}
		}
	}

	if val, ok := schedule[yesterdayKey]; ok {
		startMin, endMin, closed, valid := parseOperatingWindow(val)
		if valid && !closed && startMin > endMin && curMin < endMin {
			return true
		}
	}

	return false
}

func loadStoreLocation(timezone string) *time.Location {
	if tz := strings.TrimSpace(timezone); tz != "" {
		if loc, err := time.LoadLocation(tz); err == nil {
			return loc
		}
	}
	if loc, err := time.LoadLocation(defaultStoreTimezone); err == nil {
		return loc
	}
	return wibFixedZone
}

// parseOperatingWindow parses "HH:MM-HH:MM" into [startMin, endMin) minutes since midnight.
// Closed markers ("", "closed", "tutup", "-") return closed=true, valid=true.
func parseOperatingWindow(raw string) (startMin, endMin int, closed, valid bool) {
	v := strings.ToLower(strings.TrimSpace(raw))
	if v == "" || v == "closed" || v == "tutup" || v == "-" {
		return 0, 0, true, true
	}
	parts := strings.Split(v, "-")
	if len(parts) != 2 {
		return 0, 0, false, false
	}
	start, ok := parseClockMinutes(parts[0], false)
	if !ok {
		return 0, 0, false, false
	}
	end, ok := parseClockMinutes(parts[1], true)
	if !ok || start == end {
		return 0, 0, false, false
	}
	return start, end, false, true
}

func parseClockMinutes(raw string, allow24 bool) (int, bool) {
	parts := strings.Split(strings.TrimSpace(raw), ":")
	if len(parts) != 2 || len(parts[0]) != 2 || len(parts[1]) != 2 {
		return 0, false
	}
	h, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || h < 0 || m < 0 || m > 59 {
		return 0, false
	}
	if h == 24 && m == 0 && allow24 {
		return 24 * 60, true
	}
	if h > 23 {
		return 0, false
	}
	return h*60 + m, true
}
