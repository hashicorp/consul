package txn

import (
	"sort"

	. "gopkg.in/check.v1"
)

type DocKeySuite struct{}

var _ = Suite(&DocKeySuite{})

type T struct {
	A int
	B string
}

type T2 struct {
	A int
	B string
}

type T3 struct {
	A int
	B string
}

type T4 struct {
	A int
	B string
}

type T5 struct {
	F int
	Q string
}

type T6 struct {
	A int
	B string
}

type T7 struct {
	A bool
	B float64
}

type T8 struct {
	A int
	B string
}

type T9 struct {
	A int
	B string
	C bool
}

type T10 struct {
	C int    `bson:"a"`
	D string `bson:"b,omitempty"`
}

type T11 struct {
	C int
	D string
}

type T12 struct {
	S string
}

type T13 struct {
	p, q, r bool
	S       string
}

var docKeysTests = [][]docKeys{
	{{
		{"c", 1},
		{"c", 5},
		{"c", 2},
	}, {
		{"c", 1},
		{"c", 2},
		{"c", 5},
	}}, {{
		{"c", "foo"},
		{"c", "bar"},
		{"c", "bob"},
	}, {
		{"c", "bar"},
		{"c", "bob"},
		{"c", "foo"},
	}}, {{
		{"c", 0.2},
		{"c", 0.07},
		{"c", 0.9},
	}, {
		{"c", 0.07},
		{"c", 0.2},
		{"c", 0.9},
	}}, {{
		{"c", true},
		{"c", false},
		{"c", true},
	}, {
		{"c", false},
		{"c", true},
		{"c", true},
	}}, {{
		{"c", T{1, "b"}},
		{"c", T{1, "a"}},
		{"c", T{0, "b"}},
		{"c", T{0, "a"}},
	}, {
		{"c", T{0, "a"}},
		{"c", T{0, "b"}},
		{"c", T{1, "a"}},
		{"c", T{1, "b"}},
	}}, {{
		{"c", T{1, "a"}},
		{"c", T{0, "a"}},
	}, {
		{"c", T{0, "a"}},
		{"c", T{1, "a"}},
	}}, {{
		{"c", T3{0, "b"}},
		{"c", T2{1, "b"}},
		{"c", T3{1, "a"}},
		{"c", T2{0, "a"}},
	}, {
		{"c", T2{0, "a"}},
		{"c", T3{0, "b"}},
		{"c", T3{1, "a"}},
		{"c", T2{1, "b"}},
	}}, {{
		{"c", T5{1, "b"}},
		{"c", T4{1, "b"}},
		{"c", T5{0, "a"}},
		{"c", T4{0, "a"}},
	}, {
		{"c", T4{0, "a"}},
		{"c", T5{0, "a"}},
		{"c", T4{1, "b"}},
		{"c", T5{1, "b"}},
	}}, {{
		{"c", T6{1, "b"}},
		{"c", T7{true, 0.2}},
		{"c", T6{0, "a"}},
		{"c", T7{false, 0.04}},
	}, {
		{"c", T6{0, "a"}},
		{"c", T6{1, "b"}},
		{"c", T7{false, 0.04}},
		{"c", T7{true, 0.2}},
	}}, {{
		{"c", T9{1, "b", true}},
		{"c", T8{1, "b"}},
		{"c", T9{0, "a", false}},
		{"c", T8{0, "a"}},
	}, {
		{"c", T9{0, "a", false}},
		{"c", T8{0, "a"}},
		{"c", T9{1, "b", true}},
		{"c", T8{1, "b"}},
	}}, {{
		{"b", 2},
		{"a", 5},
		{"c", 2},
		{"b", 1},
	}, {
		{"a", 5},
		{"b", 1},
		{"b", 2},
		{"c", 2},
	}}, {{
		{"c", T11{1, "a"}},
		{"c", T11{1, "a"}},
		{"c", T10{1, "a"}},
	}, {
		{"c", T10{1, "a"}},
		{"c", T11{1, "a"}},
		{"c", T11{1, "a"}},
	}}, {{
		{"c", T12{"a"}},
		{"c", T13{false, true, false, "a"}},
		{"c", T12{"b"}},
		{"c", T13{false, true, false, "b"}},
	}, {
		{"c", T12{"a"}},
		{"c", T13{false, true, false, "a"}},
		{"c", T12{"b"}},
		{"c", T13{false, true, false, "b"}},
	}},
}

func (s *DocKeySuite) TestSort(c *C) {
	for _, test := range docKeysTests {
		keys := test[0]
		expected := test[1]
		sort.Sort(keys)
		c.Check(keys, DeepEquals, expected)
	}
}
