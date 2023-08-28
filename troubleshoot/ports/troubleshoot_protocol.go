package ports

type troubleShootProtocol interface {
	dailPort(hostPort *hostPort) string
}
