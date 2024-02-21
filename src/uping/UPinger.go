package uping

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type UPinger struct {
	conf    Conf
	running bool
	targets []*Target
	slaves  []*uPingerSlave
	report  chan bool
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

	if p.conf.Watch {
		// clear screen and move cursor up
		fmt.Printf("%c[2J", rune(033))
	}

	sentPackets := 0
	countAllUp := 0
	seedTime := time.Now()
	for p.running {
		seedTime = seedTime.Add(time.Second * time.Duration(p.conf.Interval))

		timer := time.NewTimer(time.Until(seedTime))
		<-timer.C

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
				if ok {
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

		fmt.Printf("%c[1G[%s] %c[1muPing%c[22m Int:%ds, TTL:%d, Sz:%db\n", rune(033), time.Now().Format(time.DateTime), rune(033), rune(033), p.conf.Interval, p.conf.TTL, p.conf.Size)

		for _, s := range p.slaves {
			fmt.Printf("  %c[1m%s%c[22m%s %s\n", rune(033), s.target.Host, rune(033), strings.Repeat(" ", maxTargetHostLen-len(s.target.Host)), s.Status.ToString())

			if len(p.slaves) == 1 && s.Status.State && s.Status.Seq[0] == 1 {
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
		sentPackets++

		if p.conf.Count > 0 && sentPackets >= p.conf.Count || (p.conf.CountSuccess > 0 && countAllUp >= p.conf.CountSuccess) || !childrenAreRunning {
			break
		}
	}

	// rewind last line
	//	fmt.Printf("%c[2K%c[1G", rune(033), rune(033))

	p.Stop()

	return nil
}

func (p *UPinger) Stop() {
	p.running = false
	for _, s := range p.slaves {
		s.Stop()
	}
}

func NewUPinger(conf Conf) (*UPinger, error) {
	confErrors := conf.Validate()
	if confErrors != nil {
		return nil, errors.New(strings.Join(confErrors, "\n"))

	}

	return &UPinger{conf: conf}, nil
}
