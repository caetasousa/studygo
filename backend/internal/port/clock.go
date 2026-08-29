package port

import "time"

// Clock is the source of "now". Services depend on it so tests can pin the date;
// production wires SystemClock.
type Clock interface {
	Now() time.Time
}

// SystemClock returns the real wall-clock time in UTC.
type SystemClock struct{}

func (SystemClock) Now() time.Time {
	return time.Now().UTC()
}
