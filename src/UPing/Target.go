package UPing

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
)

type Target struct {
	Host    string
	Address string
	User    string
	Port    int
}

func ParseTarget(target string) (*Target, error) {
	hostTarget := &Target{User: "", Host: "", Address: "", Port: 0}

	targetRegexp, _ := regexp.Compile("(?:(\\S+)@)?([^:]+)(?::(\\d+))?")
	if !targetRegexp.MatchString(target) {
		return nil, errors.New(fmt.Sprintf("target %s is not valid (match)", target))
	}

	matches := targetRegexp.FindStringSubmatch(target)
	if len(matches) != 4 {
		return nil, errors.New(fmt.Sprintf("target %s is not valid (find)", target))
	}

	hostTarget.Host = matches[2]

	addrs, err := net.LookupHost(matches[2])
	if err != nil {
		return nil, err
	}

	if len(addrs) == 0 {
		return nil, errors.New("cannot resolve host")
	}

	hostTarget.Address = addrs[0]

	if len(matches[1]) > 0 {
		userRegexp, _ := regexp.Compile("^[a-z_]([a-z0-9_-]{0,31}|[a-z0-9_-]{0,30}\\$)$")
		if !userRegexp.MatchString(matches[1]) {
			return nil, errors.New("target username not valid")
		}

		hostTarget.User = matches[1]
	}

	if len(matches[3]) > 0 {
		port, err := strconv.Atoi(matches[3])
		if err != nil || port < 1 || port > 65535 {
			return nil, errors.New("target port not valid")
		}

		hostTarget.Port = port
	}

	return hostTarget, nil
}
