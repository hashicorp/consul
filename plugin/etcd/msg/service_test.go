package msg

import "testing"

func TestSplit255(t *testing.T) {
	xs := split255("abc")
	if len(xs) != 1 && xs[0] != "abc" {
		t.Errorf("Failure to split abc")
	}
	s := ""
	for i := 0; i < 255; i++ {
		s += "a"
	}
	xs = split255(s)
	if len(xs) != 1 && xs[0] != s {
		t.Errorf("Failure to split 255 char long string")
	}
	s += "b"
	xs = split255(s)
	if len(xs) != 2 || xs[1] != "b" {
		t.Errorf("Failure to split 256 char long string: %d", len(xs))
	}
	for i := 0; i < 255; i++ {
		s += "a"
	}
	xs = split255(s)
	if len(xs) != 3 || xs[2] != "a" {
		t.Errorf("Failure to split 510 char long string: %d", len(xs))
	}
}

func TestGroup(t *testing.T) {
	// Key are in the wrong order, but for this test it does not matter.
	sx := Group(
		[]Service{
			{Host: "127.0.0.1", Group: "g1", Key: "b/sub/dom1/skydns/test"},
			{Host: "127.0.0.2", Group: "g2", Key: "a/dom1/skydns/test"},
		},
	)
	// Expecting to return the shortest key with a Group attribute.
	if len(sx) != 1 {
		t.Fatalf("Failure to group zeroth set: %v", sx)
	}
	if sx[0].Key != "a/dom1/skydns/test" {
		t.Fatalf("Failure to group zeroth set: %v, wrong Key", sx)
	}

	// Groups disagree, so we will not do anything.
	sx = Group(
		[]Service{
			{Host: "server1", Group: "g1", Key: "region1/skydns/test"},
			{Host: "server2", Group: "g2", Key: "region1/skydns/test"},
		},
	)
	if len(sx) != 2 {
		t.Fatalf("Failure to group first set: %v", sx)
	}

	// Group is g1, include only the top-level one.
	sx = Group(
		[]Service{
			{Host: "server1", Group: "g1", Key: "a/dom/region1/skydns/test"},
			{Host: "server2", Group: "g2", Key: "a/subdom/dom/region1/skydns/test"},
		},
	)
	if len(sx) != 1 {
		t.Fatalf("Failure to group second set: %v", sx)
	}

	// Groupless services must be included.
	sx = Group(
		[]Service{
			{Host: "server1", Group: "g1", Key: "a/dom/region1/skydns/test"},
			{Host: "server2", Group: "g2", Key: "a/subdom/dom/region1/skydns/test"},
			{Host: "server2", Group: "", Key: "b/subdom/dom/region1/skydns/test"},
		},
	)
	if len(sx) != 2 {
		t.Fatalf("Failure to group third set: %v", sx)
	}

	// Empty group on the highest level: include that one also.
	sx = Group(
		[]Service{
			{Host: "server1", Group: "g1", Key: "a/dom/region1/skydns/test"},
			{Host: "server1", Group: "", Key: "b/dom/region1/skydns/test"},
			{Host: "server2", Group: "g2", Key: "a/subdom/dom/region1/skydns/test"},
		},
	)
	if len(sx) != 2 {
		t.Fatalf("Failure to group fourth set: %v", sx)
	}

	// Empty group on the highest level: include that one also, and the rest.
	sx = Group(
		[]Service{
			{Host: "server1", Group: "g5", Key: "a/dom/region1/skydns/test"},
			{Host: "server1", Group: "", Key: "b/dom/region1/skydns/test"},
			{Host: "server2", Group: "g5", Key: "a/subdom/dom/region1/skydns/test"},
		},
	)
	if len(sx) != 3 {
		t.Fatalf("Failure to group fifth set: %v", sx)
	}

	// One group.
	sx = Group(
		[]Service{
			{Host: "server1", Group: "g6", Key: "a/dom/region1/skydns/test"},
		},
	)
	if len(sx) != 1 {
		t.Fatalf("Failure to group sixth set: %v", sx)
	}

	// No group, once service
	sx = Group(
		[]Service{
			{Host: "server1", Key: "a/dom/region1/skydns/test"},
		},
	)
	if len(sx) != 1 {
		t.Fatalf("Failure to group seventh set: %v", sx)
	}
}
