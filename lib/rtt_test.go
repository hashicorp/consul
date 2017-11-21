package lib

import (
	"math"
	"testing"
	"time"

	"github.com/hashicorp/serf/coordinate"
	"github.com/pascaldekloe/goe/verify"
)

func TestRTT_ComputeDistance(t *testing.T) {
	tests := []struct {
		desc string
		a    *coordinate.Coordinate
		b    *coordinate.Coordinate
		dist float64
	}{
		{
			"10 ms",
			GenerateCoordinate(0),
			GenerateCoordinate(10 * time.Millisecond),
			0.010,
		},
		{
			"0 ms",
			GenerateCoordinate(10 * time.Millisecond),
			GenerateCoordinate(10 * time.Millisecond),
			0.0,
		},
		{
			"2 ms",
			GenerateCoordinate(8 * time.Millisecond),
			GenerateCoordinate(10 * time.Millisecond),
			0.002,
		},
		{
			"2 ms reversed",
			GenerateCoordinate(10 * time.Millisecond),
			GenerateCoordinate(8 * time.Millisecond),
			0.002,
		},
		{
			"a nil",
			nil,
			GenerateCoordinate(8 * time.Millisecond),
			math.Inf(1.0),
		},
		{
			"b nil",
			GenerateCoordinate(8 * time.Millisecond),
			nil,
			math.Inf(1.0),
		},
		{
			"both nil",
			nil,
			nil,
			math.Inf(1.0),
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			dist := ComputeDistance(tt.a, tt.b)
			verify.Values(t, "", dist, tt.dist)
		})
	}
}

func TestRTT_Intersect(t *testing.T) {
	// The numbers here don't matter, we just want a unique coordinate for
	// each one.
	server_1 := CoordinateSet{
		"":      GenerateCoordinate(1 * time.Millisecond),
		"alpha": GenerateCoordinate(2 * time.Millisecond),
		"beta":  GenerateCoordinate(3 * time.Millisecond),
	}
	server_2 := CoordinateSet{
		"":      GenerateCoordinate(4 * time.Millisecond),
		"alpha": GenerateCoordinate(5 * time.Millisecond),
		"beta":  GenerateCoordinate(6 * time.Millisecond),
	}
	client_alpha := CoordinateSet{
		"alpha": GenerateCoordinate(7 * time.Millisecond),
	}
	client_beta_1 := CoordinateSet{
		"beta": GenerateCoordinate(8 * time.Millisecond),
	}
	client_beta_2 := CoordinateSet{
		"beta": GenerateCoordinate(9 * time.Millisecond),
	}

	tests := []struct {
		desc string
		a    CoordinateSet
		b    CoordinateSet
		c1   *coordinate.Coordinate
		c2   *coordinate.Coordinate
	}{
		{
			"nil maps",
			nil, nil,
			nil, nil,
		},
		{
			"two servers",
			server_1, server_2,
			server_1[""], server_2[""],
		},
		{
			"two clients",
			client_beta_1, client_beta_2,
			client_beta_1["beta"], client_beta_2["beta"],
		},
		{
			"server_1 and client alpha",
			server_1, client_alpha,
			server_1["alpha"], client_alpha["alpha"],
		},
		{
			"server_1 and client beta 1",
			server_1, client_beta_1,
			server_1["beta"], client_beta_1["beta"],
		},
		{
			"server_1 and client alpha reversed",
			client_alpha, server_1,
			client_alpha["alpha"], server_1["alpha"],
		},
		{
			"server_1 and client beta 1 reversed",
			client_beta_1, server_1,
			client_beta_1["beta"], server_1["beta"],
		},
		{
			"nothing in common",
			client_alpha, client_beta_1,
			nil, client_beta_1["beta"],
		},
		{
			"nothing in common reversed",
			client_beta_1, client_alpha,
			nil, client_alpha["alpha"],
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			r1, r2 := tt.a.Intersect(tt.b)
			verify.Values(t, "", r1, tt.c1)
			verify.Values(t, "", r2, tt.c2)
		})
	}
}
