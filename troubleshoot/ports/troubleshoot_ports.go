package ports

import (
	"fmt"
	"strings"
)

func TroubleshootDefaultPorts(host string) {
	ports := make(map[string][]string)
	// Source - https://developer.hashicorp.com/consul/docs/install/ports
	ports["8600"] = []string{TcpProtocol, UdpProtocol}
	ports["8500"] = []string{TcpProtocol}
	ports["8501"] = []string{TcpProtocol}
	ports["8502"] = []string{TcpProtocol, UdpProtocol}
	ports["8503"] = []string{TcpProtocol, UdpProtocol}
	ports["8301"] = []string{TcpProtocol, UdpProtocol}
	ports["8302"] = []string{TcpProtocol, UdpProtocol}
	ports["8300"] = []string{TcpProtocol}
	TroubleshootRun(ports, host)
}

func TroubleShootCustomPorts(host string, ports string) {
	portsArr := strings.Split(ports, ",")
	portsMap := make(map[string][]string)
	for _, val := range portsArr {
		portsMap[val] = []string{TcpProtocol, UdpProtocol}
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
			case TcpProtocol:
				tcpTroubleShoot := TroubleShootTcp{}
				go tcpTroubleShoot.test(&HostPort{host: host, port: port}, resultsChannel)
			case UdpProtocol:
				udpTroubleShoot := TroubleShootUdp{}
				go udpTroubleShoot.test(&HostPort{host: host, port: port}, resultsChannel)
			}
		}
	}
	for itr := 0; itr < counter; itr++ {
		fmt.Print(<-resultsChannel)
	}
}
