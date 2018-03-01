package file

// OnShutdown shuts down any running go-routines for this zone.
func (z *Zone) OnShutdown() error {
	if !z.NoReload {
		z.reloadShutdown <- true
	}
	return nil
}
