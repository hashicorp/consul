package stats

import (
	"testing"
	"github.com/v2pro/plz/countlog/core"
	"github.com/stretchr/testify/require"
	"time"
)

func Test_counter(t *testing.T) {
	should := require.New(t)
	dumpPoints := &dumpPoint{}
	aggregator := NewEventAggregator(DefaultExecutor, dumpPoints)
	counter := aggregator.HandlerOf(&spi.LogSite{
		EventOrCallee: "event!abc",
		Sample: []interface{}{
			"agg", "counter",
			"dim", "city,ver",
			"city", "beijing",
			"ver", "1.0",
		},
	}).(State)
	counter.Handle(&spi.Event{
		Properties: []interface{}{
			"agg", "counter",
			"dim", "city,ver",
			"city", "beijing",
			"ver", "1.0",
		},
	})
	counter.Handle(&spi.Event{
		Properties: []interface{}{
			"agg", "counter",
			"dim", "city,ver",
			"city", "beijing",
			"ver", "1.0",
		},
	})
	window := counter.GetWindow()
	window.Export(time.Now())
	points := *dumpPoints
	should.Equal(1, len(points))
	should.Equal(float64(2), points[0].Value)
	should.Equal([]string{"city", "beijing", "ver", "1.0"}, points[0].Dimension)
}

type dumpPoint []*Point

func (points *dumpPoint) Collect(point *Point) {
	*points = append(*points, point)
}

func Benchmark_counter_of_2_elem_dimension(b *testing.B) {
	aggregator := &EventAggregator{}
	counter := aggregator.HandlerOf(&spi.LogSite{
		EventOrCallee: "event!abc",
		Sample: []interface{}{
			"agg", "counter",
			"dim", "city,ver",
			"city", "beijing",
			"ver", "1.0",
		},
	}).(State)
	events := []*spi.Event{
		{
			Properties: []interface{}{
				"agg", "counter",
				"dim", "city,ver",
				"city", "beijing",
				"ver", "1.0",
			},
		},
		{
			Properties: []interface{}{
				"agg", "counter",
				"dim", "city,ver",
				"city", "hangzhou",
				"ver", "1.0",
			},
		},
		{
			Properties: []interface{}{
				"agg", "counter",
				"dim", "city,ver",
				"city", "hangzhou",
				"ver", "2.0",
			},
		},
		{
			Properties: []interface{}{
				"agg", "counter",
				"dim", "city,ver",
				"city", "hangzhou",
				"ver", "3.0",
			},
		},
	}
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			counter.Handle(events[i % 4])
			i++
		}
	})
	//for i := 0; i < b.N; i++ {
	//	counter.Handle(events[i % 4])
	//}
}

func Benchmark_counter_of_0_elem_dimension(b *testing.B) {
	aggregator := &EventAggregator{}
	counter := aggregator.HandlerOf(&spi.LogSite{
		EventOrCallee: "event!abc",
		Sample: []interface{}{
			"agg", "counter",
		},
	}).(State)
	event := &spi.Event{
		Properties: []interface{}{
			"agg", "counter",
		},
	}
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			counter.Handle(event)
		}
	})
	//for i := 0; i < b.N; i++ {
	//	counter.Handle(event)
	//}
}