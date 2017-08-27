package health

// Healther interface needs to be implemented by each middleware willing to
// provide healthhceck information to the health middleware. As a second step
// the middleware needs to registered against the health middleware, by addding
// it to healthers map. Note this method should return quickly, i.e. just
// checking a boolean status, as it is called every second from the health
// middleware.
type Healther interface {
	// Health returns a boolean indicating the health status of a middleware.
	// False indicates unhealthy.
	Health() bool
}

// Ok returns the global health status of all middleware configured in this server.
func (h *health) Ok() bool {
	h.RLock()
	defer h.RUnlock()
	return h.ok
}

// SetOk sets the global health status of all middleware configured in this server.
func (h *health) SetOk(ok bool) {
	h.Lock()
	defer h.Unlock()
	h.ok = ok
}

// poll polls all healthers and sets the global state.
func (h *health) poll() {
	for _, m := range h.h {
		if !m.Health() {
			h.SetOk(false)
			return
		}
	}
	h.SetOk(true)
}

// Middleware that implements the Healther interface.
// TODO(miek): none yet.
var healthers = map[string]bool{}
