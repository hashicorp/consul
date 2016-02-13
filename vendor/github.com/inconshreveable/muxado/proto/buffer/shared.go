package buffer

func BothClosed(in *Inbound, out *Outbound) (closed bool) {
	in.L.Lock()
	out.L.Lock()
	closed = (in.err != nil && out.err != nil)
	out.L.Unlock()
	in.L.Unlock()
	return
}
