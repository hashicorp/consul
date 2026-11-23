// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MPL-2.0

package lifecycle

import "time"

// Default limits. They keep memory bounded even if the Exchange is misused.
const (
	DefaultMaxComponents      = 10000
	DefaultMaxAttributes      = 16
	DefaultMaxAttributeKey    = 64
	DefaultMaxAttributeValue  = 256
	DefaultSubscriptionBuffer = 128
	DefaultObserverQueue      = 1024
	DefaultDrainTimeout       = 30 * time.Second
)

type config struct {
	logger        Logger
	metrics       Metrics
	hasLogger     bool
	hasMetrics    bool
	clock         Clock
	observers     []Observer
	drainTimeout  time.Duration
	maxComponents int
	maxAttrs      int
	maxAttrKey    int
	maxAttrValue  int
	observerQueue int
}

func defaultConfig() config {
	return config{
		logger:        noopLogger{},
		metrics:       noopMetrics{},
		clock:         realClock{},
		drainTimeout:  DefaultDrainTimeout,
		maxComponents: DefaultMaxComponents,
		maxAttrs:      DefaultMaxAttributes,
		maxAttrKey:    DefaultMaxAttributeKey,
		maxAttrValue:  DefaultMaxAttributeValue,
		observerQueue: DefaultObserverQueue,
	}
}

// Option configures an Exchange.
type Option func(*config)

// WithLogger sets the logger. It enables the built-in logging observer.
func WithLogger(l Logger) Option {
	return func(c *config) {
		if l != nil {
			c.logger = l
			c.hasLogger = true
		}
	}
}

// WithMetrics sets the metrics sink. It enables the built-in metrics observer.
func WithMetrics(m Metrics) Option {
	return func(c *config) {
		if m != nil {
			c.metrics = m
			c.hasMetrics = true
		}
	}
}

// WithObserver adds an observer. Observers are called in registration order
// from one goroutine, outside all Exchange locks.
func WithObserver(o Observer) Option {
	return func(c *config) {
		if o != nil {
			c.observers = append(c.observers, o)
		}
	}
}

// WithClock replaces the clock (tests).
func WithClock(clk Clock) Option {
	return func(c *config) {
		if clk != nil {
			c.clock = clk
		}
	}
}

// WithDrainTimeout bounds how long BeginTransition waits for leases.
func WithDrainTimeout(d time.Duration) Option {
	return func(c *config) {
		if d > 0 {
			c.drainTimeout = d
		}
	}
}

// WithMaxComponents limits the number of registered components.
func WithMaxComponents(n int) Option {
	return func(c *config) {
		if n > 0 {
			c.maxComponents = n
		}
	}
}

// WithAttributeLimits limits attributes per status. Zero keeps the default.
func WithAttributeLimits(maxKeys, maxKeyLen, maxValueLen int) Option {
	return func(c *config) {
		if maxKeys > 0 {
			c.maxAttrs = maxKeys
		}
		if maxKeyLen > 0 {
			c.maxAttrKey = maxKeyLen
		}
		if maxValueLen > 0 {
			c.maxAttrValue = maxValueLen
		}
	}
}

// WithObserverQueue sets the size of the observer queue. When the queue is
// full, notifications are dropped and counted.
func WithObserverQueue(n int) Option {
	return func(c *config) {
		if n > 0 {
			c.observerQueue = n
		}
	}
}

type transitionConfig struct {
	reason     string
	attributes map[string]string
	replace    bool
}

// TransitionOption configures a Transition, Update or BeginTransition call.
type TransitionOption func(*transitionConfig)

// WithReason records why the change happened.
func WithReason(reason string) TransitionOption {
	return func(c *transitionConfig) { c.reason = reason }
}

// WithAttributes sets attributes. By default they are merged into the current
// attributes; a value of "" deletes the key.
func WithAttributes(attrs map[string]string) TransitionOption {
	return func(c *transitionConfig) {
		if c.attributes == nil {
			c.attributes = make(map[string]string, len(attrs))
		}
		for k, v := range attrs {
			c.attributes[k] = v
		}
	}
}

// WithReplaceAttributes replaces all attributes instead of merging.
func WithReplaceAttributes() TransitionOption {
	return func(c *transitionConfig) { c.replace = true }
}

type subscribeConfig struct {
	components map[ComponentName]struct{}
	prefix     string
	buffer     int
}

func (c subscribeConfig) matches(name ComponentName) bool {
	if c.components != nil {
		if _, ok := c.components[name]; !ok {
			return false
		}
	}
	if c.prefix != "" && (len(name) < len(c.prefix) || string(name[:len(c.prefix)]) != c.prefix) {
		return false
	}
	return true
}

// SubscribeOption configures a Subscription.
type SubscribeOption func(*subscribeConfig)

// WithComponents limits the subscription to the named components.
func WithComponents(names ...ComponentName) SubscribeOption {
	return func(c *subscribeConfig) {
		if c.components == nil {
			c.components = make(map[ComponentName]struct{}, len(names))
		}
		for _, n := range names {
			c.components[n] = struct{}{}
		}
	}
}

// WithPrefix limits the subscription to components whose name has the prefix.
func WithPrefix(prefix string) SubscribeOption {
	return func(c *subscribeConfig) { c.prefix = prefix }
}

// WithBuffer sets how many events are queued for a slow subscriber before it
// is resynchronised with a fresh snapshot instead.
func WithBuffer(n int) SubscribeOption {
	return func(c *subscribeConfig) {
		if n > 0 {
			c.buffer = n
		}
	}
}
