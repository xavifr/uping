package uping

import (
	"testing"
	"time"
)

// Regression: Stop() dereferenced a nil *time.Timer, so calling it before
// Start() panicked. It must be safe and idempotent.
func TestStopBeforeStartDoesNotPanic(t *testing.T) {
	p, err := NewUPinger(NewUPingConf())
	if err != nil {
		t.Fatal(err)
	}

	p.Stop()
	p.Stop()
}

func TestStopIsIdempotent(t *testing.T) {
	p, err := NewUPinger(NewUPingConf())
	if err != nil {
		t.Fatal(err)
	}

	if err := p.AddTarget("127.0.0.1"); err != nil {
		t.Fatal(err)
	}

	// Start blocks until stopped, so it has to run in the background.
	errc := make(chan error, 1)
	go func() { errc <- p.Start() }()

	time.Sleep(200 * time.Millisecond)

	p.Stop()
	p.Stop()

	select {
	case err := <-errc:
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Start() did not return after Stop()")
	}

	if p.IsRunning() {
		t.Error("running is true after Stop()")
	}
}

// Regression: Start() blocked on the timer and was not interruptible, so Ctrl-C
// only took effect at the next tick.
func TestStartReturnsAfterStop(t *testing.T) {
	conf := NewUPingConf()
	conf.Interval = 1

	p, err := NewUPinger(conf)
	if err != nil {
		t.Fatal(err)
	}

	if err := p.AddTarget("127.0.0.1"); err != nil {
		t.Fatal(err)
	}

	go func() {
		time.Sleep(200 * time.Millisecond)
		p.Stop()
	}()

	errc := make(chan error, 1)
	go func() { errc <- p.Start() }()

	select {
	case err := <-errc:
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Start() did not return after Stop()")
	}
}

// Regression: Start() replaced the done channel it had been given by
// NewUPinger, so a Stop() racing startup closed the discarded channel and was
// never observed. Start() must return even when Stop() wins the race.
func TestStopRacingStartReturns(t *testing.T) {
	for i := 0; i < 20; i++ {
		p, err := NewUPinger(NewUPingConf())
		if err != nil {
			t.Fatal(err)
		}

		if err := p.AddTarget("127.0.0.1"); err != nil {
			t.Fatal(err)
		}

		errc := make(chan error, 1)
		go func() { errc <- p.Start() }()

		// No sleep: this is the Ctrl-C-immediately-after-launch case.
		p.Stop()

		select {
		case err := <-errc:
			if err != nil {
				t.Fatalf("Start: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("iteration %d: Stop() during startup did not interrupt Start()", i)
		}

		if p.IsRunning() {
			t.Fatalf("iteration %d: still running after Stop()", i)
		}
	}
}

func TestStartHonoursCount(t *testing.T) {
	conf := NewUPingConf()
	conf.Interval = 1
	conf.Count = 2

	p, err := NewUPinger(conf)
	if err != nil {
		t.Fatal(err)
	}

	if err := p.AddTarget("127.0.0.1"); err != nil {
		t.Fatal(err)
	}

	errc := make(chan error, 1)
	start := time.Now()
	go func() { errc <- p.Start() }()

	select {
	case err := <-errc:
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		// Two samples at a one second interval take about two seconds.
		if elapsed := time.Since(start); elapsed > 5*time.Second {
			t.Errorf("Start took %v for Count=2, expected about 2s", elapsed)
		}
	case <-time.After(10 * time.Second):
		p.Stop()
		t.Fatal("Start() did not honour Count")
	}
}

func TestNewUPingerWithNoTargetsStartsWithoutError(t *testing.T) {
	p, err := NewUPinger(NewUPingConf())
	if err != nil {
		t.Fatal(err)
	}

	// Without targets the loop has nothing to watch and should return.
	errc := make(chan error, 1)
	go func() { errc <- p.Start() }()

	select {
	case err := <-errc:
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
	case <-time.After(3 * time.Second):
		p.Stop()
		t.Fatal("Start() hung with no targets")
	}
}
