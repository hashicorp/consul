// Code generated by cmd/cgo -godefs; DO NOT EDIT.
// cgo -godefs defs_dragonfly.go

package ipv4

const (
	sysIP_RECVDSTADDR = 0x7
	sysIP_RECVIF      = 0x14
	sysIP_RECVTTL     = 0x41

	sizeofIPMreq = 0x8
)

type ipMreq struct {
	Multiaddr [4]byte /* in_addr */
	Interface [4]byte /* in_addr */
}
