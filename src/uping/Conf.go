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
	var errors []string

	if c.Source != "" && net.ParseIP(c.Source) == nil {
		errors = append(errors, "Invalid source IP Address")
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
		errors = append(errors, "Invalid audible-flags combination")
	}

	if c.Interval < 1 || c.Interval > 300 {
		errors = append(errors, "Invalid interval (1-300s)")
	}

	return errors
}

func NewUPingConf() Conf {
	return Conf{Size: 24, TTL: 64, Interval: 1, Count: -1, CountSuccess: -1}
}
