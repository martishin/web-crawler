package util

import "time"

func StartOfDay(t time.Time, loc *time.Location) time.Time {
	if loc != nil {
		t = t.In(loc)
	}
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}
