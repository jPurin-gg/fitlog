package clock

import "time"

type Clock interface {
	Now() time.Time
}

type System struct {
	location *time.Location
}

func NewSystem(location *time.Location) System {
	return System{location: location}
}

func (c System) Now() time.Time {
	return time.Now().In(c.location)
}

type Fixed struct {
	Time time.Time
}

func (c Fixed) Now() time.Time { return c.Time }
