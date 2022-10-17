package service

type Service interface {
	Terminate() error
	GetName() string
	GetAddr() (string, int)
	Start() (err error)
}
