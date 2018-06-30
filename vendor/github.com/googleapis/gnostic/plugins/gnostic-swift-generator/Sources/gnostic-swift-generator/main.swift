// Copyright 2017 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import Foundation
import Gnostic

func Log(_ message : String) {
  FileHandle.standardError.write((message + "\n").data(using:.utf8)!)
}

func main() throws {
  
  // read the code generation request
  let rawRequest = try Stdin.readall()
  let request = try Gnostic_Plugin_V1_Request(serializedData:rawRequest)

  var response = Gnostic_Plugin_V1_Response()
  
  var openapiv2 : Openapi_V2_Document?
  var surface : Surface_V1_Model?
  
  for model in request.models {
    if model.typeURL == "openapi.v2.Document" {
      openapiv2 = try Openapi_V2_Document(serializedData: model.value)      
    } else if model.typeURL == "surface.v1.Model" {
      surface = try Surface_V1_Model(serializedData: model.value)      
    }
  }  
	

  if let openapiv2 = openapiv2,
    let surface = surface {
  
    // build the service renderer
    let renderer = ServiceRenderer(surface:surface, document:openapiv2)

    // generate the desired files
    var filenames : [String]
    switch CommandLine.arguments[0] {
    case "openapi_swift_client":
      filenames = ["client.swift", "types.swift", "fetch.swift"]
    case "openapi_swift_server":
      filenames = ["server.swift", "types.swift"]
    default:
      filenames = ["client.swift", "server.swift", "types.swift", "fetch.swift"]
    }
    try renderer.generate(filenames:filenames, response:&response)
  }

  // return the results
  let serializedResponse = try response.serializedData()
  Stdout.write(bytes: serializedResponse)
}

try main()
