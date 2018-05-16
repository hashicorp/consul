package stats

import (
	"github.com/v2pro/plz/countlog/spi"
	"time"
)


type Point struct {
	Event     string
	Timestamp time.Time
	Dimension []string
	Value     float64
}

type Collector interface {
	Collect(point *Point)
}

type Monoid interface {
	Add(that Monoid)
	Export() float64
}

type State interface {
	spi.EventHandler
	GetWindow() *Window
}

type CounterMonoid uint64

func NewCounterMonoid() Monoid {
	var c CounterMonoid
	return &c
}

func (monoid *CounterMonoid) Add(that Monoid) {
	*monoid += *that.(*CounterMonoid)
}

func (monoid *CounterMonoid) Export() float64 {
	value := float64(*monoid)
	*monoid = 0
	return value
}

type MapMonoid map[interface{}]Monoid

func (monoid MapMonoid) Add(that MapMonoid) {
	for k, v := range that {
		existingV := monoid[k]
		if existingV == nil {
			monoid[k] = v
		} else {
			existingV.Add(v)
		}
	}
}