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

func TestVersionParsing(t *testing.T) {
	validVersions := []string{"0.8.2.0", "0.8.2.1", "0.9.0.0", "0.10.2.0", "1.0.0"}
	for _, s := range validVersions {
		v, err := ParseKafkaVersion(s)
		if err != nil {
			t.Errorf("could not parse valid version %s: %s", s, err)
		}
		if v.String() != s {
			t.Errorf("version %s != %s", v.String(), s)
		}
	}

	invalidVersions := []string{"0.8.2-4", "0.8.20", "1.19.0.0", "1.0.x"}
	for _, s := range invalidVersions {
		if _, err := ParseKafkaVersion(s); err == nil {
			t.Errorf("invalid version %s parsed without error", s)
		}
	}
}
