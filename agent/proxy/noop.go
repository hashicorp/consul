package proxy

// Noop implements Proxy and does nothing.
type Noop struct{}

func (p *Noop) Start() error                                   { return nil }
func (p *Noop) Stop() error                                    { return nil }
func (p *Noop) Close() error                                   { return nil }
func (p *Noop) Equal(Proxy) bool                               { return true }
func (p *Noop) MarshalSnapshot() map[string]interface{}        { return nil }
func (p *Noop) UnmarshalSnapshot(map[string]interface{}) error { return nil }
