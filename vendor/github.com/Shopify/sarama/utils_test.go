package sarama

import "testing"

func TestVersionCompare(t *testing.T) {
	if V0_8_2_0.IsAtLeast(V0_8_2_1) {
		t.Error("0.8.2.0 >= 0.8.2.1")
	}
	if !V0_8_2_1.IsAtLeast(V0_8_2_0) {
		t.Error("! 0.8.2.1 >= 0.8.2.0")
	}
	if !V0_8_2_0.IsAtLeast(V0_8_2_0) {
		t.Error("! 0.8.2.0 >= 0.8.2.0")
	}
	if !V0_9_0_0.IsAtLeast(V0_8_2_1) {
		t.Error("! 0.9.0.0 >= 0.8.2.1")
	}
	if V0_8_2_1.IsAtLeast(V0_10_0_0) {
		t.Error("0.8.2.1 >= 0.10.0.0")
	}
}
