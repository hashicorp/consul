package watch

// Chan is used to inform the server of a change. Whenever
// a watched FQDN has a change in data, that FQDN should be
// sent down this channel.
type Chan chan string

// Watchable is the interface watchable plugins should implement
type Watchable interface {
	// Name returns the plugin name.
	Name() string

	// SetWatchChan is called when the watch channel is created.
	SetWatchChan(Chan)

	// Watch is called whenever a watch is created for a FQDN. Plugins
	// should send the FQDN down the watch channel when its data may have
	// changed. This is an exact match only.
	Watch(qname string) error

	// StopWatching is called whenever all watches are canceled for a FQDN.
	StopWatching(qname string)
}
