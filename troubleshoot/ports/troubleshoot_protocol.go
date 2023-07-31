package ports

type TroubleShootProtocol interface {
	test(hostPort *HostPort, ch chan string)
}
