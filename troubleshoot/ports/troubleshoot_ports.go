package ports

import (
	"fmt"
	"strings"
)

func TroubleshootDefaultPorts(host string) {
	// Source - https://developer.hashicorp.com/consul/docs/install/ports
	ports := []string{"8600", "8500", "8501", "8502", "8503", "8301", "8302", "8300"}
	TroubleshootRun(ports, host)
}

func TroubleShootCustomPorts(host string, ports string) {
	portsArr := strings.Split(ports, ",")
	TroubleshootRun(portsArr, host)
}

func TroubleshootRun(ports []string, host string) {

	resultsChannel := make(chan string)

	var counter = 0

	tcpTroubleShoot := TroubleShootTcp{}

	for _, port := range ports {
		counter += 1
		go tcpTroubleShoot.test(&HostPort{host: host, port: port}, resultsChannel)
	}
	for itr := 0; itr < counter; itr++ {
		fmt.Print(<-resultsChannel)
	}
}
