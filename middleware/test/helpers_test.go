package test

import "testing"

func TestA(t *testing.T) {
	// should not crash
	A("miek.nl. IN A 127.0.0.1")
}
