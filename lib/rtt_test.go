// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package lib

import (
	"math"
	"testing"
	"time"

	"github.com/hashicorp/serf/coordinate"
	"github.com/stretchr/testify/require"
)

func TestRTT_ComputeDistance(t *testing.T) {
	tests := []struct {
		desc     string
		a        *coordinate.Coordinate
		b        *coordinate.Coordinate
		expected float64
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
			actual := ComputeDistance(tt.a, tt.b)
			require.Equal(t, tt.expected, actual)
		})
	}
}

func TestRTT_Intersect(t *testing.T) {
	// The numbers here don't matter, we just want a unique coordinate for
	// each one.
	server1 := CoordinateSet{
		"":      GenerateCoordinate(1 * time.Millisecond),
		"alpha": GenerateCoordinate(2 * time.Millisecond),
		"beta":  GenerateCoordinate(3 * time.Millisecond),
	}
	server2 := CoordinateSet{
		"":      GenerateCoordinate(4 * time.Millisecond),
		"alpha": GenerateCoordinate(5 * time.Millisecond),
		"beta":  GenerateCoordinate(6 * time.Millisecond),
	}
	clientAlpha := CoordinateSet{
		"alpha": GenerateCoordinate(7 * time.Millisecond),
	}
	clientBeta1 := CoordinateSet{
		"beta": GenerateCoordinate(8 * time.Millisecond),
	}
	clientBeta2 := CoordinateSet{
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
			server1, server2,
			server1[""], server2[""],
		},
		{
			"two clients",
			clientBeta1, clientBeta2,
			clientBeta1["beta"], clientBeta2["beta"],
		},
		{
			"server1 and client alpha",
			server1, clientAlpha,
			server1["alpha"], clientAlpha["alpha"],
		},
		{
			"server1 and client beta 1",
			server1, clientBeta1,
			server1["beta"], clientBeta1["beta"],
		},
		{
			"server1 and client alpha reversed",
			clientAlpha, server1,
			clientAlpha["alpha"], server1["alpha"],
		},
		{
			"server1 and client beta 1 reversed",
			clientBeta1, server1,
			clientBeta1["beta"], server1["beta"],
		},
		{
			"nothing in common",
			clientAlpha, clientBeta1,
			nil, clientBeta1["beta"],
		},
		{
			"nothing in common reversed",
			clientBeta1, clientAlpha,
			nil, clientAlpha["alpha"],
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			r1, r2 := tt.a.Intersect(tt.b)

			require.Equal(t, tt.c1, r1)
			require.Equal(t, tt.c2, r2)
		})
	}
}
