// gotestdoc parses well-formed comments in Go tests to extract BDD-like
// test specs, without the burden of some goofy DSL.

package main

import (
	"fmt"
	"go/ast"
	"go/doc/comment"
	"go/parser"
	"go/token"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/hashicorp/go-multierror"
)

type TestDoc struct {
	Givens      []comment.Block
	GivenPerms  []Perm
	Expects     []Expect
	ExpectPerms []Perm
}

type Expect struct {
	When []comment.Text
	Then []comment.Text
}

type Perm = comment.Block

const (
	kwGiven  = "Given"
	kwExpect = "Expect"
	kwPerms  = "With permutations:"
)

const (
	fsmStart       = "start"
	fsmGivenBody   = "givenBody"
	fsmGivenPerms  = "givenPerms"
	fsmExpectBody  = "expectBody"
	fsmExpectPerms = "expectPerms"
)

var whenThenRE = regexp.MustCompile("^When: (.+), then: (.+)$")

func parseExpectItem(li *comment.ListItem) (Expect, error) {
	if len(li.Content) != 1 {
		return Expect{}, fmt.Errorf("length of Expect item content must be 1; is %d", len(li.Content))
	}
	contentPara, ok := li.Content[0].(*comment.Paragraph)
	if !ok {
		return Expect{}, fmt.Errorf("Expect item must contain paragraph; is %T", li.Content[0])
	}

	// TODO: support rich text by concating Texts
	if len(contentPara.Text) != 1 {
		return Expect{}, fmt.Errorf("Expect item paragraph must be length 1; is %d", len(contentPara.Text))
	}

	contentPlain, ok := contentPara.Text[0].(comment.Plain)
	if !ok {
		return Expect{}, fmt.Errorf("Expect item paragraph must be Plain")
	}

	matches := whenThenRE.FindStringSubmatch(string(contentPlain))
	if len(matches) != 3 {
		return Expect{}, fmt.Errorf(`Expect item must be of the form "When: <...>, then": <...>; is: %q`, contentPlain)
	}

	return Expect{
		When: []comment.Text{comment.Plain(matches[1])},
		Then: []comment.Text{comment.Plain(matches[2])},
	}, nil
}

func parseFuncComment(s string) (*TestDoc, error) {
	docParser := comment.Parser{
		// TODO: links and symbols?
	}
	doc := docParser.Parse(s)
	ret := TestDoc{}
	var errs error
	// TODO; probably a better way to do FSM, but meh
	fsm := fsmStart
	for _, b := range doc.Content {
		switch v := b.(type) {
		case *comment.Heading:
			// TODO: not sure when len > 1?
			if len(v.Text) != 1 {
				errs = multierror.Append(errs, fmt.Errorf("len of heading text != 1: %#v", v))
				continue
			}
			text := v.Text[0]
			switch fsm {
			case fsmStart:
				vt, ok := text.(comment.Plain)
				if !ok {
					errs = multierror.Append(errs, fmt.Errorf("given heading should be plain; is: %#v", v))
					continue
				}
				if vt != kwGiven {
					errs = multierror.Append(errs, fmt.Errorf("given heading should be 'Given'; is: %#v", vt))
					continue
				}
				fsm = fsmGivenBody
				continue
			case fsmGivenBody, fsmGivenPerms:
				vt, ok := text.(comment.Plain)
				if !ok {
					errs = multierror.Append(errs, fmt.Errorf("Expect heading should be plain; is: %#v", v))
					continue
				}
				if vt != kwExpect {
					errs = multierror.Append(errs, fmt.Errorf("Expect heading should be 'Expect'; is: %#v", vt))
					continue
				}
				fsm = fsmExpectBody
				continue
			default:
				log.Printf("unhandled Heading state: %s", fsm)
				continue
			}
		case *comment.Paragraph:
			switch fsm {
			case fsmGivenBody:
				if p, ok := v.Text[0].(comment.Plain); ok && p == kwPerms {
					fsm = fsmGivenPerms
					continue
				}
				ret.Givens = append(ret.Givens, v)
			case fsmGivenPerms:
				// TODO: append? treat as single?
				log.Printf("unhandled Paragraph in given perms")
			case fsmExpectBody:
				if p, ok := v.Text[0].(comment.Plain); ok && p == kwPerms {
					fsm = fsmExpectPerms
					continue
				}
				errs = multierror.Append(errs, fmt.Errorf("no paragraphs in expect body"))
				continue
			case fsmExpectPerms:
				// TODO: append
				log.Printf("unhandled Paragraph in expect perms")
			default:
				log.Printf("unhandled Paragraph state: %s", fsm)
				continue
			}
		case *comment.List:
			switch fsm {
			case fsmExpectBody:
				for i, li := range v.Items {
					ex, err := parseExpectItem(li)
					if err != nil {
						errs = multierror.Append(errs, fmt.Errorf("parsing expect item %d (%q): %w", i, li.Number, err))
					}
					ret.Expects = append(ret.Expects, ex)
				}
			case fsmGivenBody:
				ret.Givens = append(ret.Givens, v)
			case fsmGivenPerms:
				ret.GivenPerms = append(ret.GivenPerms, v)
			case fsmExpectPerms:
				ret.ExpectPerms = append(ret.ExpectPerms, v)
			default:
				log.Printf("unhandled List state: %s", fsm)
				continue
			}

		default:
			switch fsm {
			case fsmGivenBody:
				ret.Givens = append(ret.Givens, v)
			case fsmGivenPerms:
				ret.GivenPerms = append(ret.GivenPerms, v)
			case fsmExpectPerms:
				ret.ExpectPerms = append(ret.ExpectPerms, v)
			default:
				log.Printf("unhandled state for misc block type: %s", fsm)
				continue
			}
			log.Printf("unhandled block type: %T", b)
			continue
		}
	}
	return &ret, errs
}

func (t *TestDoc) GoDocComment() *comment.Doc {
	cont := []comment.Block{
		&comment.Heading{
			Text: []comment.Text{
				comment.Plain(kwGiven),
			},
		},
	}
	cont = append(cont, t.Givens...)
	cont = append(cont, &comment.Paragraph{
		Text: []comment.Text{comment.Plain(kwPerms)},
	})
	cont = append(cont, []comment.Block(t.GivenPerms)...)

	cont = append(cont, &comment.Heading{
		Text: []comment.Text{
			comment.Plain(kwExpect),
		},
	})
	expectLIs := []*comment.ListItem{}
	for i, e := range t.Expects {
		text := []comment.Text{
			comment.Plain("When: "),
		}
		text = append(text, e.When...)
		text = append(text, comment.Plain(", then: "))
		text = append(text, e.Then...)
		expectLIs = append(expectLIs, &comment.ListItem{
			Number: strconv.Itoa(i + 1),
			Content: []comment.Block{
				&comment.Paragraph{
					Text: text},
			},
		})
	}
	cont = append(cont, &comment.List{Items: expectLIs})
	cont = append(cont, &comment.Paragraph{
		Text: []comment.Text{comment.Plain(kwPerms)},
	})
	cont = append(cont, []comment.Block(t.ExpectPerms)...)

	return &comment.Doc{
		Content: cont,
	}
}

func (t *TestDoc) Markdown() []byte {
	pr := comment.Printer{}
	gdc := t.GoDocComment()
	return pr.Markdown(gdc)
}

func parseFile(file *ast.File) (map[string]*TestDoc, map[string]error) {
	testDocs := map[string]*TestDoc{}
	errs := map[string]error{}

	for _, d := range file.Decls {
		switch v := d.(type) {
		case *ast.FuncDecl:
			funcname := v.Name.String()
			// TODO: also check that its signature is (t *testing.T)
			// TODO: handle duplicates?
			if strings.HasPrefix(funcname, "Test") {
				td, err := parseFuncComment(v.Doc.Text())
				if err != nil {
					errs[funcname] = err
					continue
				}
				testDocs[funcname] = td
			}
		}
	}
	return testDocs, errs
}

func main() {
	// TODO: real arg parsing
	filename := os.Args[1]
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}

	testDocs, errs := parseFile(file)

	if len(errs) > 0 {
		spew.Printf("ERRORS: %#v", errs)
	}
	// TODO; parse flags
	mode := "markdown"
	if len(errs) > 0 {
		os.Exit(1)
	}
	switch mode {
	case "dump":
		if len(testDocs) > 0 {
			spew.Dump(testDocs)
		}
	case "markdown":
		for name, doc := range testDocs {
			fmt.Printf("<h1>%s</h1>\n\n%s", name, doc.Markdown())
		}
	}

}
