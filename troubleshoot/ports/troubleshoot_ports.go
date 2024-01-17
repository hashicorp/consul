// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package ports

import (
	"fmt"
	"strings"
)

func TroubleshootDefaultPorts(host string) []string {
	// Source - https://developer.hashicorp.com/consul/docs/install/ports
	ports := []string{"8600", "8500", "8501", "8502", "8503", "8301", "8302", "8300"}
	return troubleshootRun(ports, host)
}

func TroubleShootCustomPorts(host string, ports string) []string {
	portsArr := strings.Split(ports, ",")
	return troubleshootRun(portsArr, host)
}

func troubleshootRun(ports []string, host string) []string {

	resultsChannel := make(chan string)
	defer close(resultsChannel)

	var counter = 0

	for _, port := range ports {
		counter += 1
		tcpTroubleShoot := troubleShootTcp{}
		port := port
		go func() {
			err := tcpTroubleShoot.dialPort(&hostPort{host: host, port: port})
			var res string
			if err != nil {
				res = fmt.Sprintf("TCP: Port %s on %s is closed, unreachable, or the connection timed out.\n", port, host)
			} else {
				// If no error occurs, the connection was successful, and the port is open.
				res = fmt.Sprintf("TCP: Port %s on %s is open.\n", port, host)
			}
			resultsChannel <- res
		}()
	}

	resultsArr := make([]string, counter)
	for itr := 0; itr < counter; itr++ {
		res := <-resultsChannel
		fmt.Print(res)
		resultsArr[itr] = res
	}
	return resultsArr
}
