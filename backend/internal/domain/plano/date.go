package plano

import (
	"math"
	"time"
)

// day normalizes t to midnight UTC so date arithmetic is exact.
func day(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// addDays returns t shifted by n calendar days.
func addDays(t time.Time, n int) time.Time {
	return day(t).AddDate(0, 0, n)
}

// diffDays returns the whole number of days from a to b (b - a).
func diffDays(a, b time.Time) int {
	return int(math.Round(day(b).Sub(day(a)).Hours() / 24))
}

// weekday returns 0=Sunday..6=Saturday, matching JavaScript's Date.getDay().
func weekday(t time.Time) int {
	return int(day(t).Weekday())
}

// mondayOf returns the Monday of t's ISO week.
func mondayOf(t time.Time) time.Time {
	offset := (weekday(t) + 6) % 7

	return addDays(t, -offset)
}

// sameDay reports whether a and b fall on the same calendar day.
func sameDay(a, b time.Time) bool {
	return day(a).Equal(day(b))
}

// DayOf normalizes t to midnight UTC. Exported for the service and worker.
func DayOf(t time.Time) time.Time {
	return day(t)
}

// DiffDays returns the whole number of days from a to b (b - a).
func DiffDays(a, b time.Time) int {
	return diffDays(a, b)
}

// AddDays returns t shifted by n calendar days, normalized to midnight UTC.
func AddDays(t time.Time, n int) time.Time {
	return addDays(t, n)
}

// findDia localiza o dia do plano numa data, ou nil.
func findDia(dias []Dia, dt time.Time) *Dia {
	for i := range dias {
		if sameDay(dias[i].Data, dt) {
			return &dias[i]
		}
	}

	return nil
}
