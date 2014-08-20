package watch

import (
	"fmt"
	"reflect"
	"testing"
)

func TestTokenize(t *testing.T) {
	type tcase struct {
		in  string
		out []*token
		err error
	}
	cases := []tcase{
		tcase{
			"",
			nil,
			nil,
		},
		tcase{
			"foo:bar bar:baz	zip:zap",
			[]*token{
				&token{"foo", "bar"},
				&token{"bar", "baz"},
				&token{"zip", "zap"},
			},
			nil,
		},
		tcase{
			"foo:\"long input here\" after:this",
			[]*token{
				&token{"foo", "long input here"},
				&token{"after", "this"},
			},
			nil,
		},
		tcase{
			"foo:'long input here' after:this",
			[]*token{
				&token{"foo", "long input here"},
				&token{"after", "this"},
			},
			nil,
		},
		tcase{
			"foo:'long input here after:this",
			nil,
			fmt.Errorf("Missing end of quotation"),
		},
		tcase{
			"foo",
			nil,
			fmt.Errorf("Parameter delimiter not found"),
		},
		tcase{
			":val",
			nil,
			fmt.Errorf("Missing parameter name"),
		},
	}

	for _, tc := range cases {
		tokens, err := tokenize(tc.in)
		if err != nil && tc.err == nil {
			t.Fatalf("%s: err: %v", tc.in, err)
		} else if tc.err != nil && (err == nil || err.Error() != tc.err.Error()) {
			t.Fatalf("%s: bad err: %v", tc.in, err)
		}
		if !reflect.DeepEqual(tokens, tc.out) {
			t.Fatalf("%s: bad: %#v %#v", tc.in, tokens, tc.out)
		}
	}
}

func TestCollapse(t *testing.T) {
	inp := "type:key key:foo key:bar"
	tokens, err := tokenize(inp)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	out := collapse(tokens)
	expect := map[string][]string{
		"type": []string{"key"},
		"key":  []string{"foo", "bar"},
	}
	if !reflect.DeepEqual(out, expect) {
		t.Fatalf("bad: %#v", out)
	}
}

func TestParseBasic(t *testing.T) {
	p, err := Parse("type:key datacenter:dc2 token:12345 key:foo")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if p.Datacenter != "dc2" {
		t.Fatalf("Bad: %#v", p)
	}
	if p.Token != "12345" {
		t.Fatalf("Bad: %#v", p)
	}
	if p.Type != "key" {
		t.Fatalf("Bad: %#v", p)
	}
}

func TestParse_exempt(t *testing.T) {
	p, err := ParseExempt("type:key key:foo handler:foobar", []string{"handler"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if p.Type != "key" {
		t.Fatalf("Bad: %#v", p)
	}
	ex := p.Exempt["handler"]
	if len(ex) != 1 && ex[0] != "foobar" {
		t.Fatalf("bad: %v", ex)
	}
}
