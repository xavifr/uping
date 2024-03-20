package uping

import (
	"fmt"
	"github.com/hairyhenderson/go-which"
	probing "github.com/prometheus-community/pro-bing"
	"os"
	"os/exec"
	"time"
)

type uPingerSlave struct {
	conf    Conf
	target  *Target
	pinger  *probing.Pinger
	Rtt     chan time.Duration
	running bool
	paused  bool
	Status  *UPingerSlaveStatus
}

func (u *uPingerSlave) Run() {
	u.running = true
	go func() {
		for u.running {
			if u.preparePinger() {
				u.pinger.Run()
			}

			time.Sleep(time.Second * time.Duration(u.conf.Interval))
		}

		u.running = false
	}()
}

func (u *uPingerSlave) Stop() {
	u.running = false
	if u.pinger != nil {
		u.pinger.Stop()
	}
}

func (u *uPingerSlave) IsRunning() bool {
	return u.running
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
	u.paused = true
	startCmd := time.Now()
	cmd.Run()

	timeElapsed := time.Since(startCmd)
	u.Status.Seq[0] += int(timeElapsed.Seconds())
	u.paused = false
}

func (u *uPingerSlave) preparePinger() bool {
	if u.pinger != nil {
		u.pinger.Stop()
	}

	pinger := probing.New(u.target.Address)

	privileged, err := checkPrivileged()
	if err != nil {
		u.pinger = nil
		return false
	}

	pinger.SetPrivileged(privileged)
	pinger.SetLogger(nil)

	pinger.RecordRtts = false

	// conf to pinger
	if u.conf.Size != 0 {
		pinger.Size = u.conf.Size
	}

	pinger.Interval = time.Second * time.Duration(u.conf.Interval)
	/*if conf.Count != -1 {
		slave.pinger.Count = conf.Count
	}*/

	if u.conf.Source != "" {
		pinger.Source = u.conf.Source
	}

	if u.conf.TTL != 0 {
		pinger.TTL = u.conf.TTL
	}

	pinger.OnRecv = func(pkt *probing.Packet) {
		if !u.paused && pkt.Rtt < time.Second*time.Duration(u.conf.Interval) {
			u.Rtt <- pkt.Rtt
		}
	}

	u.pinger = pinger
	return true
}

func newUPingerSlave(conf Conf, target *Target) (*uPingerSlave, error) {
	slave := &uPingerSlave{conf: conf, target: target, Status: newUPingerSlaveStatus()}

	slave.Rtt = make(chan time.Duration, 1)

	return slave, nil
}
