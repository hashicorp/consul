package metrics

import "testing"

func TestForEachTodo(t *testing.T) {
	a, i := newAddress(), 0
	a.setAddress("test", func() error { i++; return nil })

	a.forEachTodo()
	if i != 1 {
		t.Errorf("Failed to executed f for %s", "test")
	}
	a.forEachTodo()
	if i != 1 {
		t.Errorf("Executed f twice instead of once")
	}
}
