package uping

import (
	"fmt"
	"time"
)

type UPingStatus bool

const (
	Down UPingStatus = false
	Up               = true
)

type UPingerSlaveStatus struct {
	State   UPingStatus
	LastRtt time.Duration
	Seq     []int

	AvgRtt    time.Duration
	TotalOK   int
	TotalSent int
}

func (s *UPingerSlaveStatus) AppendUp(rtt time.Duration) (changed bool) {
	if s.State == Down || len(s.Seq) == 0 {
		s.State = Up
		s.Seq = append([]int{0}, s.Seq...)
		changed = true
	}

	s.Seq[0]++
	s.LastRtt = rtt
	s.TotalOK++
	s.TotalSent++

	s.gc()

	return
}

func (s *UPingerSlaveStatus) AppendDown() (changed bool) {
	if s.State || len(s.Seq) == 0 {
		s.State = Down
		s.Seq = append([]int{0}, s.Seq...)
		changed = true
	}

	s.Seq[0]++
	s.TotalSent++

	s.gc()

	return
}

func (s *UPingerSlaveStatus) GetPercent() float64 {
	if s.TotalSent == 0 || s.TotalOK == 0 {
		return 0
	}

	return float64(s.TotalOK*100) / float64(s.TotalSent)
}

func (s *UPingerSlaveStatus) GetSequence() string {
	message := ""
	state := s.State
	for i, seq := range s.Seq {
		colorBase := 30
		if i == 0 {
			colorBase += 60
		} else {
			message = fmt.Sprintf("%s / ", message)
		}

		if state {
			message = fmt.Sprintf("%s%c[%dm%d%c[39m", message, rune(033), colorBase+2, seq, rune(033))
		} else {
			message = fmt.Sprintf("%s%c[%dm%d%c[39m", message, rune(033), colorBase+1, seq, rune(033))
		}

		state = !state
	}

	return message
}

func (s *UPingerSlaveStatus) ToString() string {
	message := ""
	if s.State {
		message = fmt.Sprintf("%c[1;32m UP %c[22;39m [ %s ] (%.1fms %.1f%%)", rune(033), rune(033), s.GetSequence(), float64(s.LastRtt.Microseconds())/float64(1000), s.GetPercent())
	} else {
		message = fmt.Sprintf("%c[1;31mDOWN%c[22;39m [ %s ] (%.1f%%)", rune(033), rune(033), s.GetSequence(), s.GetPercent())
	}

	return message
}

func (s *UPingerSlaveStatus) gc() {
	if len(s.Seq) > 5 {
		s.Seq = s.Seq[0:5]
	}
}
