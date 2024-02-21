//go:build !windows

package uping

import (
	"errors"
	"github.com/lorenzosaino/go-sysctl"
	"kernel.org/pub/linux/libs/security/libcap/cap"
)

func checkPrivileged() (bool, error) {
	proc := cap.GetProc()
	if proc != nil {
		capEnabled, _ := proc.GetFlag(cap.Effective|cap.Permitted, cap.NET_RAW)
		if capEnabled {
			return true, nil
		} else {
			ctl, _ := sysctl.Get("net.ipv4.ping_group_range")
			if ctl == "1\t0" {
				return false, errors.New("not enough privileges to send pings")
			}
		}
	}

	return false, nil
}
