package ports

type TroubleShootProtocol interface {
	test(hostPort *hostPort, ch chan string)
}
