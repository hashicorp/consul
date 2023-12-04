// NB: tests in gotestdoc are described with gotestdoc-style comments, in order to dogfood.
// In real, life, this would probably be overkill.

package main

import (
	"go/doc/comment"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// # Given
//
// A basic gotestdoc-style comment.
//
// # Expect
//
//  1. When: it is parsed, then: it will have no errors
//  2. When: it is parsed, then: the results will be a TestDoc struct with a particular form
func TestBasic(t *testing.T) {
	s := `
# Given

givenbody

With permutations:

  1. givenperm

# Expect

  1. When: foo, then: bar

With permutations:

  1. expectperm
	`
	d, err := parseFuncComment(s)
	e := TestDoc{
		Givens: []comment.Block{&comment.Paragraph{
			Text: []comment.Text{
				comment.Plain("givenbody"),
			},
		}},
		GivenPerms: []comment.Block{&comment.List{
			ForceBlankBefore: true,
			Items: []*comment.ListItem{
				{
					Number: "1",
					Content: []comment.Block{
						&comment.Paragraph{
							Text: []comment.Text{comment.Plain("givenperm")},
						},
					},
				},
			},
		}},
		Expects: []Expect{
			{
				When: []comment.Text{
					comment.Plain("foo"),
				},
				Then: []comment.Text{
					comment.Plain("bar"),
				},
			},
		},
		ExpectPerms: []comment.Block{&comment.List{
			ForceBlankBefore: true,
			Items: []*comment.ListItem{
				{
					Number: "1",
					Content: []comment.Block{
						&comment.Paragraph{
							Text: []comment.Text{comment.Plain("expectperm")},
						},
					},
				},
			},
		}},
	}
	require.NoError(t, err)
	assert.EqualValues(t, &e, d)
}
