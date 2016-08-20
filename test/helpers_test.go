package test

import "testing"

func TestA(t *testing.T) { A("miek.nl. IN A 127.0.0.1") } // should not crash
