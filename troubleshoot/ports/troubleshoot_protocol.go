package ports

type troubleShootProtocol interface {
	dialPort(hostPort *hostPort) error
}
