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

	var counter = 0

	for _, port := range ports {
		counter += 1
		tcpTroubleShoot := troubleShootTcp{}
		port := port
		go func() {
			res := tcpTroubleShoot.dailPort(&hostPort{host: host, port: port})
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
