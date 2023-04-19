package test

import (
	"fmt"
	"testing"
)

func TestMatrix(t *testing.T) {
	matrix := NewMatrix()
	for i := 0; i < 10; i++ {
		fmt.Printf("===============================================>%v\n\n\n", i)
		consul, vault, more := matrix.NextPair(t)
		if !more {
			return
		}
		t.Run("demo", func(t *testing.T) {
			demo(t, consul, vault)
		})
		consul.Stop()
		vault.Stop()
	}
}

type Matrix struct {
	consulVersions, vaultVersions []string
	pairs                         []pair
}

func NewMatrix() Matrix {
	cvs := latestReleases("consul")
	vvs := latestReleases("vault")
	pairs := make([]pair, 0, 9)
	for _, cv := range cvs {
		for _, vv := range vvs {
			pairs = append(pairs, pair{vault: vv, consul: cv})
		}
	}
	return Matrix{
		consulVersions: cvs,
		vaultVersions:  vvs,
		pairs:          pairs,
	}
}

// for each consul test with each vault (3x3 matrix, 9 test cases)
func (m Matrix) NextPair(t *testing.T) (TestConsulServer, TestVaultServer, bool) {
	nextPair := m.next()
	if nextPair.Nil() {
		return TestConsulServer{}, TestVaultServer{}, false
	}
	//	return NewTestConsulServer(t, getBinary("consul", nextPair.consul)),
	//		TestVaultServer{}, true
	return NewTestConsulServer(t, getBinary("consul", nextPair.consul)),
		NewTestVaultServer(t, getBinary("vault", nextPair.vault)), true
}

type pair struct {
	vault, consul string
}

func (p pair) Nil() bool {
	return p.consul == "" || p.vault == ""
}

func (m Matrix) next() pair {
	for i, p := range m.pairs {
		if !p.Nil() {
			m.pairs[i] = pair{}
			return p
		}
	}
	return pair{}
}
