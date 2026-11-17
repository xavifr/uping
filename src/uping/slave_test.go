package uping

import (
	"context"
	"testing"
	"time"
)

// These tests pin the behaviour of the critical fixes: a slave must produce one
// sample per interval, must never block its sender, and must stop promptly.

func newLoopbackSlave(t *testing.T) *uPingerSlave {
	t.Helper()

	conf := NewUPingConf()
	conf.Interval = 1

	s, err := newUPingerSlave(conf, &Target{Host: "localhost", Address: "127.0.0.1"})
	if err != nil {
		t.Fatalf("newUPingerSlave: %v", err)
	}

	return s
}

// Regression: Run() used to call pinger.Run() with the library defaults
// (Count=-1, Timeout=MaxInt64), so the loop never came back and a slave emitted
// exactly one sample for the whole process lifetime.
func TestRunEmitsOneSamplePerInterval(t *testing.T) {
	s := newLoopbackSlave(t)

	if _, err := s.probe(); err != nil {
		t.Skipf("cannot ping on this host: %v", err)
	}

	s.Run()
	defer s.Stop()

	const interval = time.Second
	const watch = 3500 * time.Millisecond

	deadline := time.After(watch)
	samples := 0

	for {
		select {
		case <-deadline:
			// ~3 samples expected in 3.5 intervals; allow some slack for CI.
			if samples < 2 || samples > 4 {
				t.Fatalf("got %d samples in %v at interval=%v, want about 3", samples, watch, interval)
			}
			return
		case <-s.Rtt:
			samples++
		}
	}
}

func TestRunStopsPromptly(t *testing.T) {
	s := newLoopbackSlave(t)
	s.Run()

	time.Sleep(100 * time.Millisecond)

	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		s.Stop()
	}()

	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop() blocked")
	}

	if s.IsRunning() {
		t.Error("IsRunning() is true after Stop()")
	}

	// A stopped slave must not deliver new samples. A probe already inside
	// report() when Stop() lands is unavoidable, so drain and then require the
	// channel to stay quiet.
drain:
	for {
		select {
		case <-s.Rtt:
		case <-time.After(200 * time.Millisecond):
			break drain
		}
	}

	select {
	case rtt := <-s.Rtt:
		t.Fatalf("received %v after Stop()", rtt)
	case <-time.After(2 * time.Second):
	}
}

// Regression: OnRecv wrote to a size-1 channel with no select, so a full buffer
// stalled pro-bing's receive goroutine.
func TestReportNeverBlocks(t *testing.T) {
	s := newLoopbackSlave(t)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 10000; i++ {
			s.report(time.Millisecond, true)
		}
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("report() blocked; the pinger goroutine would stall")
	}
}

func TestReportKeepsNewestSample(t *testing.T) {
	s := newLoopbackSlave(t)

	s.report(time.Millisecond, true)
	s.report(2*time.Millisecond, true)

	if got := <-s.Rtt; got != 2*time.Millisecond {
		t.Errorf("kept %v, want the newest sample (2ms)", got)
	}
}

func TestReportEncodesTimeoutAsNegative(t *testing.T) {
	s := newLoopbackSlave(t)

	s.report(0, false)

	if got := <-s.Rtt; got >= 0 {
		t.Errorf("timeout sample = %v, want a negative value", got)
	}
}

// Regression: checkPrivileged/OnSendError failures produced no sample at all,
// so a misconfiguration looked identical to packet loss.
func TestProbeSurfacesSetupErrors(t *testing.T) {
	conf := NewUPingConf()
	conf.Interval = 1
	conf.Size = 1 // below pro-bing's minimum of 24

	s, err := newUPingerSlave(conf, &Target{Host: "localhost", Address: "127.0.0.1"})
	if err != nil {
		t.Fatalf("newUPingerSlave: %v", err)
	}

	_, err = s.probe()
	if err == nil {
		t.Fatal("expected an error for an undersized packet")
	}
}

func TestProbeUnroutableSourceIsAnError(t *testing.T) {
	conf := NewUPingConf()
	conf.Interval = 1
	conf.Source = "192.0.2.222" // TEST-NET-1, not configured on this host

	s, err := newUPingerSlave(conf, &Target{Host: "localhost", Address: "127.0.0.1"})
	if err != nil {
		t.Fatalf("newUPingerSlave: %v", err)
	}

	if _, err := s.probe(); err == nil {
		t.Fatal("expected an error for an unusable source address")
	}
}

func TestProbeLoopbackSucceeds(t *testing.T) {
	s := newLoopbackSlave(t)

	rtt, err := s.probe()
	if err != nil {
		t.Skipf("cannot ping on this host: %v", err)
	}

	if rtt < 0 {
		t.Fatalf("rtt = %v, want a non-negative round-trip time", rtt)
	}
}

func TestProbeHonoursContextCancellation(t *testing.T) {
	s := newLoopbackSlave(t)

	// Cancel before probing so the pinger cannot run to completion.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s.ctx = ctx

	done := make(chan struct{})
	go func() {
		defer close(done)
		s.probe()
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("probe() ignored context cancellation")
	}
}
