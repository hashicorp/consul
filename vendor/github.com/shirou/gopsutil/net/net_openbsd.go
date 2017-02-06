// +build openbsd

package net

import (
	"errors"
	"os/exec"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/internal/common"
)

func ParseNetstat(output string, mode string,
	iocs map[string]IOCountersStat) error {
	lines := strings.Split(output, "\n")

	exists := make([]string, 0, len(lines)-1)

	columns := 6
	if mode == "ind" {
		columns = 10
	}
	for _, line := range lines {
		values := strings.Fields(line)
		if len(values) < 1 || values[0] == "Name" {
			continue
		}
		if common.StringsHas(exists, values[0]) {
			// skip if already get
			continue
		}

		if len(values) < columns {
			continue
		}
		base := 1
		// sometimes Address is ommitted
		if len(values) < columns {
			base = 0
		}

		parsed := make([]uint64, 0, 8)
		var vv []string
		if mode == "inb" {
			vv = []string{
				values[base+3], // BytesRecv
				values[base+4], // BytesSent
			}
		} else {
			vv = []string{
				values[base+3], // Ipkts
				values[base+4], // Ierrs
				values[base+5], // Opkts
				values[base+6], // Oerrs
				values[base+8], // Drops
			}
		}
		for _, target := range vv {
			if target == "-" {
				parsed = append(parsed, 0)
				continue
			}

			t, err := strconv.ParseUint(target, 10, 64)
			if err != nil {
				return err
			}
			parsed = append(parsed, t)
		}
		exists = append(exists, values[0])

		n, present := iocs[values[0]]
		if !present {
			n = IOCountersStat{Name: values[0]}
		}
		if mode == "inb" {
			n.BytesRecv = parsed[0]
			n.BytesSent = parsed[1]
		} else {
			n.PacketsRecv = parsed[0]
			n.Errin = parsed[1]
			n.PacketsSent = parsed[2]
			n.Errout = parsed[3]
			n.Dropin = parsed[4]
			n.Dropout = parsed[4]
		}

		iocs[n.Name] = n
	}
	return nil
}

func IOCounters(pernic bool) ([]IOCountersStat, error) {
	netstat, err := exec.LookPath("/usr/bin/netstat")
	if err != nil {
		return nil, err
	}
	out, err := invoke.Command(netstat, "-inb")
	if err != nil {
		return nil, err
	}
	out2, err := invoke.Command(netstat, "-ind")
	if err != nil {
		return nil, err
	}
	iocs := make(map[string]IOCountersStat)

	lines := strings.Split(string(out), "\n")
	ret := make([]IOCountersStat, 0, len(lines)-1)

	err = ParseNetstat(string(out), "inb", iocs)
	if err != nil {
		return nil, err
	}
	err = ParseNetstat(string(out2), "ind", iocs)
	if err != nil {
		return nil, err
	}

	for _, ioc := range iocs {
		ret = append(ret, ioc)
	}

	if pernic == false {
		return getIOCountersAll(ret)
	}

	return ret, nil
}

// NetIOCountersByFile is an method which is added just a compatibility for linux.
func IOCountersByFile(pernic bool, filename string) ([]IOCountersStat, error) {
	return IOCounters(pernic)
}

func FilterCounters() ([]FilterStat, error) {
	return nil, errors.New("NetFilterCounters not implemented for openbsd")
}

// NetProtoCounters returns network statistics for the entire system
// If protocols is empty then all protocols are returned, otherwise
// just the protocols in the list are returned.
// Not Implemented for OpenBSD
func ProtoCounters(protocols []string) ([]ProtoCountersStat, error) {
	return nil, errors.New("NetProtoCounters not implemented for openbsd")
}

// Return a list of network connections opened.
// Not Implemented for OpenBSD
func Connections(kind string) ([]ConnectionStat, error) {
	return nil, errors.New("Connections not implemented for openbsd")
}
