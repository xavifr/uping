package uping

import (
	"strings"
	"testing"
	"time"
)

func TestAppendUpFromDown(t *testing.T) {
	s := newUPingerSlaveStatus()

	if !s.AppendUp(time.Millisecond) {
		t.Fatal("first sample after a down state must report a change")
	}

	if s.State != Up {
		t.Errorf("State = %v, want Up", s.State)
	}

	if len(s.Seq) != 1 || s.Seq[0] != 1 {
		t.Errorf("Seq = %v, want [1]", s.Seq)
	}

	if s.LastRtt != time.Millisecond {
		t.Errorf("LastRtt = %v, want 1ms", s.LastRtt)
	}

	if s.TotalOK != 1 || s.TotalSent != 1 {
		t.Errorf("totals = (%d, %d), want (1, 1)", s.TotalOK, s.TotalSent)
	}
}

func TestAppendUpConsecutiveDoesNotChange(t *testing.T) {
	s := newUPingerSlaveStatus()
	s.AppendUp(time.Millisecond)

	if s.AppendUp(2 * time.Millisecond) {
		t.Error("consecutive success must not report a change")
	}

	if s.Seq[0] != 2 {
		t.Errorf("Seq[0] = %d, want 2", s.Seq[0])
	}
}

func TestAppendUpRttStatistics(t *testing.T) {
	s := newUPingerSlaveStatus()

	s.AppendUp(10 * time.Millisecond)
	s.AppendUp(20 * time.Millisecond)
	s.AppendUp(30 * time.Millisecond)

	if s.MinRtt != 10*time.Millisecond {
		t.Errorf("MinRtt = %v, want 10ms", s.MinRtt)
	}

	if s.MaxRtt != 30*time.Millisecond {
		t.Errorf("MaxRtt = %v, want 30ms", s.MaxRtt)
	}

	if s.LastRtt != 30*time.Millisecond {
		t.Errorf("LastRtt = %v, want 30ms", s.LastRtt)
	}

	if s.AvgRtt != 20*time.Millisecond {
		t.Errorf("AvgRtt = %v, want 20ms", s.AvgRtt)
	}
}

func TestAppendDownFromUp(t *testing.T) {
	s := newUPingerSlaveStatus()
	s.AppendUp(time.Millisecond)

	if !s.AppendDown() {
		t.Fatal("first sample after an up state must report a change")
	}

	if s.State != Down {
		t.Errorf("State = %v, want Down", s.State)
	}

	if len(s.Seq) != 2 || s.Seq[0] != 1 {
		t.Errorf("Seq = %v, want a new run [1 ...]", s.Seq)
	}

	if s.TotalSent != 2 || s.TotalOK != 1 {
		t.Errorf("totals = (%d ok, %d sent), want (1, 2)", s.TotalOK, s.TotalSent)
	}
}

func TestAppendDownFromInitialState(t *testing.T) {
	s := newUPingerSlaveStatus()

	// The zero value already means "down", but there is no run recorded yet.
	if !s.AppendDown() {
		t.Fatal("a down sample with no history must report a change")
	}

	if len(s.Seq) != 1 || s.Seq[0] != 1 {
		t.Errorf("Seq = %v, want [1]", s.Seq)
	}
}

func TestSeqIsCappedAtFive(t *testing.T) {
	s := newUPingerSlaveStatus()

	for i := 0; i < 20; i++ {
		s.AppendUp(time.Millisecond)
		s.AppendDown()
	}

	if len(s.Seq) > 5 {
		t.Errorf("len(Seq) = %d, want at most 5", len(s.Seq))
	}
}

func TestGetPercent(t *testing.T) {
	tests := []struct {
		name     string
		up, down int
		want     float64
	}{
		{name: "no samples", up: 0, down: 0, want: 0},
		{name: "only losses", up: 0, down: 4, want: 0},
		{name: "all ok", up: 4, down: 0, want: 100},
		{name: "half", up: 1, down: 1, want: 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newUPingerSlaveStatus()

			for i := 0; i < tt.up; i++ {
				s.AppendUp(time.Millisecond)
			}

			for i := 0; i < tt.down; i++ {
				s.AppendDown()
			}

			if got := s.GetPercent(); got != tt.want {
				t.Errorf("GetPercent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSkipDoesNotRecordALoss(t *testing.T) {
	s := newUPingerSlaveStatus()
	s.AppendUp(time.Millisecond)

	okBefore, sentBefore := s.TotalOK, s.TotalSent

	s.Skip(30)

	if s.State != Up {
		t.Errorf("State = %v, want Up after a skip", s.State)
	}

	if s.Seq[0] != 31 {
		t.Errorf("Seq[0] = %d, want 31 (1 sample + 30 skipped seconds)", s.Seq[0])
	}

	if s.TotalOK != okBefore || s.TotalSent != sentBefore {
		t.Errorf("totals changed by Skip: (%d, %d) -> (%d, %d)", okBefore, sentBefore, s.TotalOK, s.TotalSent)
	}
}

func TestSkipIgnoredWhenNotSampledYet(t *testing.T) {
	s := newUPingerSlaveStatus()

	s.Skip(10)

	if len(s.Seq) != 0 {
		t.Errorf("Seq = %v, want empty", s.Seq)
	}
}

func TestSkipIgnoresNonPositive(t *testing.T) {
	s := newUPingerSlaveStatus()
	s.AppendUp(time.Millisecond)

	s.Skip(0)
	s.Skip(-5)

	if s.Seq[0] != 1 {
		t.Errorf("Seq[0] = %d, want 1", s.Seq[0])
	}
}

func TestToStringReportsStateAndRate(t *testing.T) {
	s := newUPingerSlaveStatus()
	s.AppendUp(1500 * time.Microsecond)

	up := s.ToString()
	if !strings.Contains(up, "UP") {
		t.Errorf("ToString() = %q, want it to contain UP", up)
	}

	if !strings.Contains(up, "100.0%") {
		t.Errorf("ToString() = %q, want it to contain 100.0%%", up)
	}

	if !strings.Contains(up, "1.5ms") {
		t.Errorf("ToString() = %q, want the last RTT (1.5ms)", up)
	}

	s.AppendDown()

	down := s.ToString()
	if !strings.Contains(down, "DOWN") {
		t.Errorf("ToString() = %q, want it to contain DOWN", down)
	}

	if !strings.Contains(down, "50.0%") {
		t.Errorf("ToString() = %q, want it to contain 50.0%%", down)
	}
}

func TestGetSequenceMarksRunLengths(t *testing.T) {
	s := newUPingerSlaveStatus()
	s.AppendUp(time.Millisecond)
	s.AppendUp(time.Millisecond)
	s.AppendDown()

	// Two successes followed by one loss: "2 / 1".
	seq := s.GetSequence()
	if !strings.Contains(seq, "2") || !strings.Contains(seq, "1") {
		t.Errorf("GetSequence() = %q, want it to show both run lengths", seq)
	}
}
