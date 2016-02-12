package lib

import (
	"testing"
)

func TestMathMaxInt(t *testing.T) {
	tests := [][3]int{{1, 2, 2}, {-1, 1, 1}, {2, 0, 2}}
	for i, _ := range tests {
		expected := tests[i][2]
		actual := MaxInt(tests[i][0], tests[i][1])
		if expected != actual {
			t.Fatalf("in iteration %d expected %d, got %d", i, expected, actual)
		}
	}
}

func TestMathMinInt(t *testing.T) {
	tests := [][3]int{{1, 2, 1}, {-1, 1, -1}, {2, 0, 0}}
	for i, _ := range tests {
		expected := tests[i][2]
		actual := MinInt(tests[i][0], tests[i][1])
		if expected != actual {
			t.Fatalf("in iteration %d expected %d, got %d", i, expected, actual)
		}
	}
}
