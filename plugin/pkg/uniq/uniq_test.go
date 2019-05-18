package uniq

import "testing"

func TestForEach(t *testing.T) {
	u, i := New(), 0
	u.Set("test", func() error { i++; return nil })

	u.ForEach()
	if i != 1 {
		t.Errorf("Failed to executed f for %s", "test")
	}
	u.ForEach()
	if i != 1 {
		t.Errorf("Executed f twice instead of once")
	}
}
