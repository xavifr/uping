package uping

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

type UPinger struct {
	conf    Conf
	running bool
	targets []*Target
	slaves  []*uPingerSlave
	report  chan bool
	timer   *time.Timer
	done    chan struct{}
	once    sync.Once
}

func (p *UPinger) AddTarget(target string) error {
	if p.running {
		return errors.New("cannot add target while running")
	}

	t, err := ParseTarget(target)
	if err != nil {
		return err
	}

	p.targets = append(p.targets, t)

	return nil
}

func (p *UPinger) Start() error {
	maxTargetHostLen := 0

	p.done = make(chan struct{})

	p.slaves = make([]*uPingerSlave, 0)
	for _, target := range p.targets {
		slave, err := newUPingerSlave(p.conf, target)
		if err != nil {
			return err
		}
		p.slaves = append(p.slaves, slave)

		if len(target.Host) > maxTargetHostLen {
			maxTargetHostLen = len(target.Host)
		}

	}

	p.running = true
	for _, s := range p.slaves {
		s.Run()
	}

	// Fail loudly when pinging is impossible instead of looping forever with
	// every target reported as down.
	if err := p.checkPrivileges(); err != nil {
		p.Stop()
		return err
	}

	if p.conf.Watch {
		// clear screen and move cursor up
		fmt.Printf("%c[2J", rune(033))
	}

	sentPackets := 0
	countAllUp := 0
	seedTime := time.Now()
	for p.running {
		seedTime = seedTime.Add(time.Second * time.Duration(p.conf.Interval))

		p.timer = time.NewTimer(time.Until(seedTime))
		select {
		case <-p.timer.C:
		case <-p.done:
		}

		sentPackets++

		childrenAreRunning := false
		childrenChanged := false

		oneUp := false
		oneDown := false
		allUp := true
		allDown := true

		for _, slave := range p.slaves {
			if slave.IsRunning() {
				childrenAreRunning = true
			}

			select {
			case rtt, ok := <-slave.Rtt:
				if ok && rtt >= 0 {
					allDown = false
					oneUp = true
					childrenChanged = slave.Status.AppendUp(rtt) || childrenChanged
				} else {
					oneDown = true
					allUp = false
					childrenChanged = slave.Status.AppendDown() || childrenChanged
				}
			default:
				oneDown = true
				allUp = false
				childrenChanged = slave.Status.AppendDown() || childrenChanged
			}
		}

		if !p.running && !childrenAreRunning {
			break
		}

		if p.conf.Watch {
			// clear screen
			fmt.Printf("%c[1J%c[1;1H", rune(033), rune(033))
		} else if !childrenChanged {
			// rewind and clean printed lines
			for range p.slaves {
				fmt.Printf("%c[1A", rune(033))
				fmt.Printf("%c[2K", rune(033))
			}
			fmt.Printf("%c[1A", rune(033))
			fmt.Printf("%c[2K", rune(033))
		}

		if p.conf.AudibleSingle && oneUp || p.conf.AudibleAll && allUp || p.conf.ZudibleSingle && oneDown || p.conf.ZudibleAll && allDown {
			fmt.Printf("%c", rune(007))
		}

		fmt.Printf("%c[1G[%s] %c[1muPing%c[22m int:%ds, sent:%d\n", rune(033), time.Now().Format(time.DateTime), rune(033), rune(033), p.conf.Interval, sentPackets)

		for _, s := range p.slaves {
			line := s.Status.ToString()

			// A probe that never reached the wire is a configuration error, not
			// packet loss, so say so instead of printing a bare DOWN.
			if err := s.LastError(); err != nil {
				line = fmt.Sprintf("%c[1;31mERR%c[22;39m %s", rune(033), rune(033), err)
			}

			fmt.Printf("  %c[1m%s%c[22m%s %s\n", rune(033), s.target.Host, rune(033), strings.Repeat(" ", maxTargetHostLen-len(s.target.Host)), line)

			if len(p.slaves) == 1 && s.Status.State && len(s.Status.Seq) > 0 && s.Status.Seq[0] == 1 {
				if p.conf.ExecSSH {
					s.execSSH()
					seedTime = time.Now()
				}
			}
		}

		if allUp {
			countAllUp++
		} else {
			countAllUp = 0
		}

		if p.conf.Count > 0 && sentPackets >= p.conf.Count || (p.conf.CountSuccess > 0 && countAllUp >= p.conf.CountSuccess) {
			break
		}

		if !childrenAreRunning {
			break
		}
	}

	p.Stop()

	return nil
}

// checkPrivileges surfaces a configuration problem that would otherwise look
// like every target being down.
func (p *UPinger) checkPrivileges() error {
	_, err := checkPrivileged()
	return err
}

func (p *UPinger) Stop() {
	p.once.Do(func() {
		if p.done != nil {
			close(p.done)
		}
	})

	p.running = false
	for _, s := range p.slaves {
		s.Stop()
	}

	if p.timer != nil {
		p.timer.Stop()
	}
}

func NewUPinger(conf Conf) (*UPinger, error) {
	confErrors := conf.Validate()
	if confErrors != nil {
		return nil, errors.New(strings.Join(confErrors, "\n"))

	}

	return &UPinger{conf: conf, done: make(chan struct{})}, nil
}
