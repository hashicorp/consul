package kubernetes

// Health implements the health.Healther interface.
func (k *Kubernetes) Health() bool { return k.APIConn.HasSynced() }
