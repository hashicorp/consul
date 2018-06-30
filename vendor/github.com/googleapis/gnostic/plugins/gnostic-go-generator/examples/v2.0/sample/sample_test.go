/*
 Copyright 2018 Google Inc. All Rights Reserved.

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

	"github.com/googleapis/gnostic/plugins/gnostic-go-generator/examples/v2.0/sample/sample"
)

const service = "http://localhost:8080"

func TestSample(t *testing.T) {
	// create a client
	s := sample.NewClient(service, nil)
	// verify a sample request
	{
		message := "hello world"
		response, err := s.GetSample(message)
		if err != nil {
			t.Log("get sample failed")
			t.Fail()
		}
		if response.OK.Id != message || response.OK.Count != int32(len(message)) {
			t.Log(fmt.Sprintf("get sample received %+v", response.OK))
			t.Fail()
		}
		if (response == nil) || (response.OK == nil) {
			t.Log(fmt.Sprintf("get sample failed %+v", response.OK))
			t.Fail()
		}
	}
	// verify the handling of an invalid request
	{
		req, err := http.NewRequest("GET", service+"/unsupported", strings.NewReader(""))
		if err != nil {
			t.Log("bad request failed")
			return
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return
		}
		// we expect a 404 (Not Found) code
		if resp.StatusCode != 404 {
			t.Log("bad request failed")
			t.Fail()
		}
		return
	}
}
