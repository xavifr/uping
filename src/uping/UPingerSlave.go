package uping

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/hairyhenderson/go-which"
	probing "github.com/prometheus-community/pro-bing"
)

type uPingerSlave struct {
	conf   Conf
	target *Target

	mu      sync.Mutex
	pinger  *probing.Pinger
	running bool
	lastErr error

	// Rtt receives exactly one value per interval: the round-trip time when the
	// echo was answered, or a negative value when it was not.
	Rtt    chan time.Duration
	Status *UPingerSlaveStatus

	ctx    context.Context
	cancel context.CancelFunc
}

// Run starts the sampling loop. Each iteration sends a single echo request and
// waits for its reply or a timeout, so the slave reports one result per
// interval whether or not the target answers.
func (u *uPingerSlave) Run() {
	u.mu.Lock()
	u.running = true
	u.mu.Unlock()

	go func() {
		for u.IsRunning() {
			start := time.Now()

			rtt, err := u.probe()

			u.mu.Lock()
			u.lastErr = err
			u.mu.Unlock()

			if u.IsRunning() {
				if err != nil {
					// Not a sample: the probe never got off the wire.
				} else if rtt < 0 {
					u.report(0, false)
				} else {
					u.report(rtt, true)
				}
			}

			// Fixed cadence: time spent probing counts towards the interval.
			if elapsed := time.Since(start); elapsed < u.interval() {
				select {
				case <-time.After(u.interval() - elapsed):
				case <-u.ctx.Done():
					return
				}
			}
		}
	}()
}

// report delivers a sample without blocking. If the consumer is behind, the
// oldest pending sample is discarded rather than stalling the sender. A reply
// is encoded as rtt >= 0; a timeout as a negative value.
func (u *uPingerSlave) report(rtt time.Duration, ok bool) {
	sample := rtt
	if !ok {
		sample = -1
	}

	select {
	case u.Rtt <- sample:
		return
	default:
	}

	select {
	case <-u.Rtt:
	default:
	}

	select {
	case u.Rtt <- sample:
	default:
	}
}

func (u *uPingerSlave) interval() time.Duration {
	return time.Second * time.Duration(u.conf.Interval)
}

func (u *uPingerSlave) Stop() {
	u.mu.Lock()
	u.running = false
	pinger := u.pinger
	u.mu.Unlock()

	if u.cancel != nil {
		u.cancel()
	}

	if pinger != nil {
		pinger.Stop()
	}
}

func (u *uPingerSlave) IsRunning() bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.running
}

// LastError reports the most recent probe failure, if any.
func (u *uPingerSlave) LastError() error {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.lastErr
}

func (u *uPingerSlave) execSSH() {
	sshPath := which.Which("ssh")
	if len(sshPath) == 0 {
		fmt.Println("SSH path not found")
		return
	}

	var cmdArgs []string

	if u.target.Port != 0 {
		cmdArgs = append(cmdArgs, "-p", fmt.Sprintf("%d", u.target.Port))
	}

	if len(u.target.User) > 0 {
		cmdArgs = append(cmdArgs, "-l", u.target.User)
	}

	cmdArgs = append(cmdArgs, u.target.Host)

	cmd := exec.Command(sshPath, cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	fmt.Printf("Connecting to %s\n", u.target.Host)
	startCmd := time.Now()
	err := cmd.Run()
	if err != nil {
		fmt.Printf("SSH to %s failed: %s\n", u.target.Host, err)
		return
	}

	// Do not count the time spent in the session as lost samples.
	u.Status.Skip(int(time.Since(startCmd).Seconds()))
}

// probe sends a single echo request and waits for its reply or the deadline.
func (u *uPingerSlave) probe() (time.Duration, error) {
	privileged, err := checkPrivileged()
	if err != nil {
		return 0, err
	}

	pinger := probing.New(u.target.Address)

	pinger.SetPrivileged(privileged)
	pinger.SetLogger(nil)

	pinger.RecordRtts = false
	pinger.Count = 1
	pinger.Interval = u.interval()
	pinger.Timeout = u.interval()

	if u.conf.Size != 0 {
		pinger.Size = u.conf.Size
	}

	if u.conf.Source != "" {
		pinger.Source = u.conf.Source
	}

	if u.conf.TTL != 0 {
		pinger.TTL = u.conf.TTL
	}

	var (
		rtt     time.Duration
		got     bool
		sendErr error
	)

	pinger.OnRecv = func(pkt *probing.Packet) {
		rtt = pkt.Rtt
		got = true
	}

	pinger.OnSendError = func(_ *probing.Packet, err error) {
		sendErr = err
	}

	u.mu.Lock()
	u.pinger = pinger
	u.mu.Unlock()

	runErr := pinger.RunWithContext(u.ctx)

	u.mu.Lock()
	u.pinger = nil
	u.mu.Unlock()

	if got {
		return rtt, nil
	}

	// A send failure means the probe never reached the wire (bad source, no
	// permission, oversized packet). Report it so the UI can distinguish a
	// configuration error from simple packet loss.
	if sendErr != nil {
		return 0, sendErr
	}

	if runErr != nil && !errors.Is(runErr, context.Canceled) {
		return 0, runErr
	}

	// Nothing came back within the deadline: the target is simply down. Signal
	// that with a negative RTT rather than an error.
	return -1, nil
}

func newUPingerSlave(conf Conf, target *Target) (*uPingerSlave, error) {
	ctx, cancel := context.WithCancel(context.Background())

	slave := &uPingerSlave{
		conf:   conf,
		target: target,
		Status: newUPingerSlaveStatus(),
		ctx:    ctx,
		cancel: cancel,
	}

	slave.Rtt = make(chan time.Duration, 1)

	return slave, nil
}
