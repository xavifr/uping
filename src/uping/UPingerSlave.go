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
		err := u.pinger.Run()
		if err != nil {
			fmt.Println(err)
		}
		u.running = false
	}()
}

func (u *uPingerSlave) Stop() {
	u.running = false
	u.pinger.Stop()
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

func newUPingerSlave(conf Conf, target *Target) (*uPingerSlave, error) {
	slave := &uPingerSlave{conf: conf, target: target, Status: newUPingerSlaveStatus()}

	slave.Rtt = make(chan time.Duration, 1)
	slave.pinger = probing.New(target.Address)

	privileged, err := checkPrivileged()
	if err != nil {
		return nil, err
	}

	slave.pinger.SetPrivileged(privileged)

	// conf to pinger
	if conf.Size != 0 {
		slave.pinger.Size = conf.Size
	}

	slave.pinger.Interval = time.Second * time.Duration(slave.conf.Interval)
	if conf.Count != -1 {
		slave.pinger.Count = conf.Count
	}

	if conf.Source != "" {
		slave.pinger.Source = conf.Source
	}

	if conf.TTL != 0 {
		slave.pinger.TTL = conf.TTL
	}

	slave.pinger.OnRecv = func(pkt *probing.Packet) {
		if !slave.paused && pkt.Rtt < time.Second*time.Duration(slave.conf.Interval) {
			slave.Rtt <- pkt.Rtt
		}
	}

	slave.pinger.OnFinish = func(statistics *probing.Statistics) {
		slave.running = false
		slave.pinger.Stop()
	}

	return slave, nil
}
