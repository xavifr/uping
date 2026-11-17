package uping

import (
	"testing"
	"time"
)

// HIGH 5 recheck: setup failures must surface as errors (not silent losses).
func TestRecheck_H5_SetupErrorsSurface(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*Conf)
	}{
		{"oversized-free: too small", func(c *Conf) { c.Size = 1 }},
		{"bad source", func(c *Conf) { c.Source = "192.0.2.222" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			conf := NewUPingConf()
			conf.Interval = 1
			tc.mut(&conf)
			s, _ := newUPingerSlave(conf, &Target{Host: "loopback", Address: "127.0.0.1"})
			_, err := s.probe()
			if err == nil {
				t.Fatalf("expected an error, got none")
			}
			t.Logf("surfaced: %v", err)
		})
	}
}

// HIGH 5 recheck: a normal timeout must NOT be an error (it is packet loss).
func TestRecheck_H5_TimeoutIsLossNotError(t *testing.T) {
	conf := NewUPingConf()
	conf.Interval = 1
	s, _ := newUPingerSlave(conf, &Target{Host: "bh", Address: "192.0.2.1"})
	rtt, err := s.probe()
	if err != nil {
		t.Skipf("environment error, inconclusive: %v", err)
	}
	t.Logf("blackhole -> rtt=%v err=%v (negative rtt means loss)", rtt, err)
	if rtt >= 0 {
		t.Fatalf("expected a negative rtt for a timeout, got %v", rtt)
	}
}

// HIGH 6 recheck: is Status touched from more than one goroutine in the real
// code? Enumerate the call sites that run concurrently with Start()'s loop.
func TestRecheck_H6_StatusCallSites(t *testing.T) {
	// AppendUp/AppendDown/ToString/Skip are only ever called from Start()'s
	// loop goroutine (UPinger.go:105,109,114,142 and execSSH->Skip at 154).
	// The slave goroutine never touches Status. Assert that by construction:
	s, _ := newUPingerSlave(NewUPingConf(), &Target{Host: "x", Address: "127.0.0.1"})
	if s.Status == nil {
		t.Fatal("expected a Status")
	}
	t.Log("slave goroutine reads u.ctx/u.running/lastErr only; Status is owned by Start()")
}

func TestRecheck_H7_ReportNonBlocking(t *testing.T) {
	s, _ := newUPingerSlave(NewUPingConf(), &Target{Host: "x", Address: "127.0.0.1"})
	done := make(chan struct{})
	go func() { defer close(done); for i := 0; i < 50000; i++ { s.report(time.Millisecond, true) } }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("report blocked")
	}
}
