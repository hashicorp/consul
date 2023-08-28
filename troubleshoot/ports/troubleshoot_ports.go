package ports

import (
	"fmt"
	"strings"
)

func TroubleshootDefaultPorts(host string) {
	ports := make(map[string][]string)
	// Source - https://developer.hashicorp.com/consul/docs/install/ports
	ports["8600"] = []string{TCP_PROTOCOL, UDP_PROTOCOL}
	ports["8500"] = []string{TCP_PROTOCOL}
	ports["8501"] = []string{TCP_PROTOCOL}
	ports["8502"] = []string{GRPC_PROTOCOL}
	ports["8503"] = []string{GRPC_PROTOCOL}
	ports["8301"] = []string{TCP_PROTOCOL, UDP_PROTOCOL}
	ports["8302"] = []string{TCP_PROTOCOL, UDP_PROTOCOL}
	ports["8300"] = []string{TCP_PROTOCOL}
	TroubleshootRun(ports, host)
}

func TroubleShootCustomPorts(host string, ports string) {
	portsArr := strings.Split(ports, ",")
	portsMap := make(map[string][]string)
	for _, val := range portsArr {
		portsMap[val] = []string{TCP_PROTOCOL, UDP_PROTOCOL, GRPC_PROTOCOL}
	}
	TroubleshootRun(portsMap, host)
}

func TroubleshootRun(ports map[string][]string, host string) {

	resultsChannel := make(chan string)

	var counter = 0

	for port, _ := range ports {
		for _, protocol := range ports[port] {
			counter += 1
			switch protocol {
			case TCP_PROTOCOL:
				tcpTroubleShoot := TroubleShootTcp{}
				go tcpTroubleShoot.test(&HostPort{host: host, port: port}, resultsChannel)
				break
			case UDP_PROTOCOL:
				udpTroubleShoot := TroubleShootUdp{}
				go udpTroubleShoot.test(&HostPort{host: host, port: port}, resultsChannel)
				break
			case GRPC_PROTOCOL:
				grpcTroubleShoot := TroubleShootGrpc{}
				go grpcTroubleShoot.test(&HostPort{host: host, port: port}, resultsChannel)
				break
			}
		}
	}
	for itr := 0; itr < counter; itr++ {
		fmt.Print(<-resultsChannel)
	}
}
