package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"uping/uping"
)

var versionString = "3.0"
var usage = `
Usage:

    ping [ -vw ] [ -aAzZ ][-I <source>] [-b <size>] [-i <seconds>] [-t ttl] [-c/C <count>] [ -sS ] <target> [<target> ...]

    -a: Audible ping, sound bell if one target answers
    -A: Audible ping, sound bell if all targets answers
    -z: Inverse Audible ping, sound bell if one target fails
    -Z: Inverse Audible ping, sound bell if all targets fails

    -I: Source interface/ip
    -b: Packet size in bytes (default: 24)
    -i: Send packets with <seconds> interval (default: 1)
    -t: Packet Time-To-Live (default by os)

    -c: Exit after sending <count> packets (default without limit)
    -C: Exit after <count> successfull packets in a row (default disabled)

    -s: Execute SSH command on target on success (only one target)

    -v: Show version and exit
    -w: Clear screen at defined intervals

	<target>: (<user>@)?<host>(:<port>)? (max 16 targets)

`

func isIP(ipString string) bool {
	return net.ParseIP(ipString) != nil
}

func getIPFromInterface(intName string) string {
	inter, err := net.InterfaceByName(intName)
	if err != nil {
		return ""
	}

	addrs, err := inter.Addrs()
	if err != nil || len(addrs) == 0 {
		return ""
	}

	switch ip := addrs[0].(type) {
	case *net.IPNet:
		return ip.IP.String()
	case *net.IPAddr:
		return ip.IP.String()
	}

	return ""
}

func main() {
	audibleSingle := flag.Bool("a", false, "")
	audibleAll := flag.Bool("A", false, "")

	size := flag.Int("b", 24, "")

	count := flag.Int("c", -1, "")
	countSuccess := flag.Int("C", -1, "")

	execSSH := flag.Bool("s", false, "")

	interval := flag.Int("i", 1, "")
	source := flag.String("I", "", "")

	ttl := flag.Int("t", 64, "TTL")

	zudibleSingle := flag.Bool("z", false, "")
	zudibleAll := flag.Bool("Z", false, "")

	version := flag.Bool("v", false, "")
	watch := flag.Bool("w", false, "")

	flag.Usage = func() {
		fmt.Print(usage)
	}
	flag.Parse()

	if *version {
		fmt.Println(versionString)
		return
	}

	if flag.NArg() == 0 || flag.NArg() > 16 {
		flag.Usage()
		return
	}

	conf := uping.NewUPingConf()

	conf.AudibleSingle = *audibleSingle
	conf.ZudibleSingle = *zudibleSingle
	conf.AudibleAll = *audibleAll
	conf.ZudibleAll = *zudibleAll

	conf.Size = *size
	conf.TTL = *ttl

	conf.Count = *count
	conf.CountSuccess = *countSuccess

	conf.ExecSSH = *execSSH

	conf.Watch = *watch

	if len(*source) > 0 {
		if isIP(*source) {
			conf.Source = *source
		} else if getIPFromInterface(*source) != "" {
			conf.Source = getIPFromInterface(*source)
		} else {
			flag.Usage()
			fmt.Println("Error: Invalid source address")
			return
		}
	}

	if interval != nil {
		conf.Interval = *interval
	}

	if conf.ExecSSH && flag.NArg() > 1 {
		flag.Usage()
		fmt.Println("Error: Execution flag is only compatible with 1-target mode")
		return
	}

	pinger, err := uping.NewUPinger(conf)
	if err != nil {
		flag.Usage()
		fmt.Println("Errors:")
		fmt.Println(err)
		return
	}

	for _, arg := range flag.Args() {
		err := pinger.AddTarget(arg)
		if err != nil {
			flag.Usage()
			fmt.Printf("Error: Invalid target %s\n", arg)
			return
		}
	}

	// listen for ctrl-C signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for range c {
			pinger.Stop()
		}
	}()

	err = pinger.Start()
	if err != nil {
		fmt.Println(err)
	}

}
