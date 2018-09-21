package testing

import (
	"fmt"
	"net/http"
	"reflect"
	"testing"

	"github.com/gophercloud/gophercloud/pagination"
	"github.com/gophercloud/gophercloud/testhelper"
)

// LinkedPager sample and test cases.

type LinkedPageResult struct {
	pagination.LinkedPageBase
}

func (r LinkedPageResult) IsEmpty() (bool, error) {
	is, err := ExtractLinkedInts(r)
	return len(is) == 0, err
}

func ExtractLinkedInts(r pagination.Page) ([]int, error) {
	var s struct {
		Ints []int `json:"ints"`
	}
	err := (r.(LinkedPageResult)).ExtractInto(&s)
	return s.Ints, err
}

func createLinked(t *testing.T) pagination.Pager {
	testhelper.SetupHTTP()

	testhelper.Mux.HandleFunc("/page1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		fmt.Fprintf(w, `{ "ints": [1, 2, 3], "links": { "next": "%s/page2" } }`, testhelper.Server.URL)
	})

	testhelper.Mux.HandleFunc("/page2", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		fmt.Fprintf(w, `{ "ints": [4, 5, 6], "links": { "next": "%s/page3" } }`, testhelper.Server.URL)
	})

	testhelper.Mux.HandleFunc("/page3", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		fmt.Fprintf(w, `{ "ints": [7, 8, 9], "links": { "next": null } }`)
	})

	client := createClient()

	createPage := func(r pagination.PageResult) pagination.Page {
		return LinkedPageResult{pagination.LinkedPageBase{PageResult: r}}
	}

	return pagination.NewPager(client, testhelper.Server.URL+"/page1", createPage)
}

func TestEnumerateLinked(t *testing.T) {
	pager := createLinked(t)
	defer testhelper.TeardownHTTP()

	callCount := 0
	err := pager.EachPage(func(page pagination.Page) (bool, error) {
		actual, err := ExtractLinkedInts(page)
		if err != nil {
			return false, err
		}

		t.Logf("Handler invoked with %v", actual)

		var expected []int
		switch callCount {
		case 0:
			expected = []int{1, 2, 3}
		case 1:
			expected = []int{4, 5, 6}
		case 2:
			expected = []int{7, 8, 9}
		default:
			t.Fatalf("Unexpected call count: %d", callCount)
			return false, nil
		}

		if !reflect.DeepEqual(expected, actual) {
			t.Errorf("Call %d: Expected %#v, but was %#v", callCount, expected, actual)
		}

		callCount++
		return true, nil
	})
	if err != nil {
		t.Errorf("Unexpected error for page iteration: %v", err)
	}

	if callCount != 3 {
		t.Errorf("Expected 3 calls, but was %d", callCount)
	}
}

func TestAllPagesLinked(t *testing.T) {
	pager := createLinked(t)
	defer testhelper.TeardownHTTP()

	page, err := pager.AllPages()
	testhelper.AssertNoErr(t, err)

	expected := []int{1, 2, 3, 4, 5, 6, 7, 8, 9}
	actual, err := ExtractLinkedInts(page)
	testhelper.AssertNoErr(t, err)
	testhelper.CheckDeepEquals(t, expected, actual)
}
