/*
 Copyright 2017 Google Inc. All Rights Reserved.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/googleapis/gnostic/plugins/gnostic-go-generator/examples/v3.0/bookstore/bookstore"
)

const service = "http://localhost:8080"

//const service = "http://generated-bookstore.appspot.com"

func TestBookstore(t *testing.T) {
	// create a client
	b := bookstore.NewClient(service, nil)
	// reset the service by deleting all shelves
	{
		err := b.DeleteShelves()
		if err != nil {
			t.Log("delete shelves failed")
			t.Fail()
		}
	}
	// verify that the service has no shelves
	{
		response, err := b.ListShelves()
		if err != nil {
			t.Log("list shelves failed")
			t.Fail()
		}
		if (response == nil) || (response.OK == nil) || (response.OK.Shelves != nil) {
			t.Log(fmt.Sprintf("list shelves failed %+v", response.OK))
			t.Log(fmt.Sprintf("list shelves failed len=%d", len(response.OK.Shelves)))
			t.Fail()
		}
	}
	// attempting to get a shelf should return an error
	{
		response, err := b.GetShelf(1)
		if err == nil {
			t.Logf("get shelf failed to return an error (%+v)", response.OK)
			t.Fail()
		}
	}
	// attempting to get a book should return an error
	{
		response, err := b.GetBook(1, 2)
		if err == nil {
			t.Logf("get book failed to return an error (%+v)", response.OK)
			t.Fail()
		}
	}
	// add a shelf
	{
		var shelf bookstore.Shelf
		shelf.Theme = "mysteries"
		response, err := b.CreateShelf(shelf)
		if err != nil {
			t.Log("create shelf mysteries failed")
			t.Fail()
		}
		if (response.OK.Name != "shelves/1") ||
			(response.OK.Theme != "mysteries") {
			t.Log("create shelf mysteries failed")
			t.Fail()
		}
	}
	// add another shelf
	{
		var shelf bookstore.Shelf
		shelf.Theme = "comedies"
		response, err := b.CreateShelf(shelf)
		if err != nil {
			t.Log("create shelf comedies failed")
			t.Fail()
		}
		if (response.OK.Name != "shelves/2") ||
			(response.OK.Theme != "comedies") {
			t.Log("create shelf comedies failed")
			t.Fail()
		}
	}
	// get the first shelf that was added
	{
		response, err := b.GetShelf(1)
		if err != nil {
			t.Log("get shelf mysteries failed")
			t.Fail()
		}
		if (response.OK.Name != "shelves/1") ||
			(response.OK.Theme != "mysteries") {
			t.Log("get shelf mysteries failed")
			t.Fail()
		}
	}
	// list shelves and verify that there are 2
	{
		response, err := b.ListShelves()
		if err != nil {
			t.Log("list shelves failed")
			t.Fail()
		}
		if len(response.OK.Shelves) != 2 {
			t.Log("list shelves failed")
			t.Fail()
		}
	}
	// delete a shelf
	{
		err := b.DeleteShelf(2)
		if err != nil {
			t.Log("delete shelf failed")
			t.Fail()
		}
	}
	// list shelves and verify that there is only 1
	{
		response, err := b.ListShelves()
		if err != nil {
			t.Log("list shelves failed")
			t.Fail()
		}
		if len(response.OK.Shelves) != 1 {
			t.Log("list shelves failed")
			t.Fail()
		}
	}
	// list books on a shelf, verify that there are none
	{
		response, err := b.ListBooks(1)
		if err != nil {
			t.Log("list books failed")
			t.Fail()
		}
		if len(response.OK.Books) != 0 {
			t.Log("list books failed")
			t.Fail()
		}
	}
	// create a book
	{
		var book bookstore.Book
		book.Author = "Agatha Christie"
		book.Title = "And Then There Were None"
		_, err := b.CreateBook(1, book)
		if err != nil {
			t.Log("create book failed")
			t.Fail()
		}
	}
	// create another book
	{
		var book bookstore.Book
		book.Author = "Agatha Christie"
		book.Title = "Murder on the Orient Express"
		_, err := b.CreateBook(1, book)
		if err != nil {
			t.Log("create book failed")
			t.Fail()
		}
	}
	// get the first book that was added
	{
		_, err := b.GetBook(1, 1)
		if err != nil {
			t.Log("get book failed")
			t.Fail()
		}
	}
	// list the books on a shelf and verify that there are 2
	{
		response, err := b.ListBooks(1)
		if err != nil {
			t.Log("list books failed")
			t.Fail()
		}
		if len(response.OK.Books) != 2 {
			t.Log("list books failed")
			t.Fail()
		}
	}
	// delete a book
	{
		err := b.DeleteBook(1, 2)
		if err != nil {
			t.Log("delete book failed")
			t.Fail()
		}
	}
	// list the books on a shelf and verify that is only 1
	{
		response, err := b.ListBooks(1)
		if err != nil {
			t.Log("list books failed")
			t.Fail()
		}
		if len(response.OK.Books) != 1 {
			t.Log("list books failed")
			t.Fail()
		}
	}
	// verify the handling of a badly-formed request
	{
		req, err := http.NewRequest("POST", service+"/shelves", strings.NewReader(""))
		if err != nil {
			t.Log("bad request failed")
			return
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return
		}
		// we expect a 400 (Bad Request) code
		if resp.StatusCode != 400 {
			t.Log("bad request failed")
			t.Fail()
		}
		return
	}
}
