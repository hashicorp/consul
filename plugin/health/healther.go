package health

// Healther interface needs to be implemented by each plugin willing to
// provide healthhceck information to the health plugin. As a second step
// the plugin needs to registered against the health plugin, by addding
// it to healthers map. Note this method should return quickly, i.e. just
// checking a boolean status, as it is called every second from the health
// plugin.
type Healther interface {
	// Health returns a boolean indicating the health status of a plugin.
	// False indicates unhealthy.
	Health() bool
}

// Ok returns the global health status of all plugin configured in this server.
func (h *health) Ok() bool {
	h.RLock()
	defer h.RUnlock()
	return h.ok
}

// SetOk sets the global health status of all plugin configured in this server.
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

// Plugins that implements the Healther interface.
// TODO(miek): none yet.
var healthers = map[string]bool{}
