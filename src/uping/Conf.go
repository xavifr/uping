package uping

import (
	"net"
)

type Conf struct {
	AudibleSingle, AudibleAll bool
	ZudibleSingle, ZudibleAll bool

	Size int

	Count, CountSuccess int

	ExecSSH bool

	Interval int

	Source string
	TTL    int

	Watch bool
}

func (c *Conf) Validate() []string {
	var problems []string

	if c.Source != "" && net.ParseIP(c.Source) == nil {
		problems = append(problems, "Invalid source IP Address")
	}

	audibles := 0
	if c.AudibleAll {
		audibles++
	}
	if c.AudibleSingle {
		audibles++
	}
	if c.ZudibleAll {
		audibles++
	}
	if c.ZudibleSingle {
		audibles++
	}

	if audibles > 1 {
		problems = append(problems, "Invalid audible-flags combination")
	}

	if c.Interval < 1 || c.Interval > 300 {
		problems = append(problems, "Invalid interval (1-300s)")
	}

	// A negative size or TTL would only surface later, as a send error on every
	// tick, so reject it up front.
	if c.Size < 0 {
		problems = append(problems, "Invalid packet size (must be >= 0)")
	}

	if c.TTL < 0 {
		problems = append(problems, "Invalid TTL (must be >= 0)")
	}

	// -1 means unlimited for the count flags; anything below that is a typo.
	if c.Count < -1 || c.CountSuccess < -1 {
		problems = append(problems, "Invalid count (must be >= -1)")
	}

	return problems
}

func NewUPingConf() Conf {
	return Conf{Size: 24, Interval: 1, Count: -1, CountSuccess: -1}
}
