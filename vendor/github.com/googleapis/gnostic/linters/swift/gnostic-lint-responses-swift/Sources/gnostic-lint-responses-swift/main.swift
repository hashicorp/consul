// Copyright 2018 Google Inc. All Rights Reserved.
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

extension Gnostic_Plugin_V1_Response {
  mutating func message(level:Gnostic_Plugin_V1_Message.Level,
                        code:String,
                        text:String,
                        path:[String]=[]) {
    var message = Gnostic_Plugin_V1_Message()
    message.level = level
    message.code = code
    message.text = text
    message.keys = path
    messages.append(message)
  }
}

class ResponseLinter {
  var document : Openapi_V2_Document = Openapi_V2_Document()

  func run(_ request : Gnostic_Plugin_V1_Request,
           _ response : inout Gnostic_Plugin_V1_Response) throws {
    for model in request.models {
      if model.typeURL == "openapi.v2.Document" {
        let document = try Openapi_V2_Document(serializedData: model.value)
        self.document = document
        for pair in document.paths.path {
          let path = ["paths", pair.name]
          let v = pair.value
          if v.hasGet {
            checkOperation(v.get, path:path + ["get"], response:&response)
          }
          if v.hasPost {
            checkOperation(v.post, path:path + ["post"], response:&response)
          }
          if v.hasPut {
            checkOperation(v.put, path:path + ["put"], response:&response)
          }
          if v.hasDelete {
            checkOperation(v.delete, path:path + ["delete"], response:&response)
          }
        }
      }
    }
  }

  func checkOperation(_ operation:Openapi_V2_Operation,
                      path:[String],
                      response:inout Gnostic_Plugin_V1_Response) {
    for responseCode in operation.responses.responseCode {
      let code = responseCode.name
      if responseCode.value.response.hasSchema {
        var schema = responseCode.value.response.schema.schema
        if schema.ref != "" {
          if let resolvedSchema = resolveReference(schema.ref) {
            schema = resolvedSchema
          }
        }
        checkSchemaType(schema, path: path + ["responses", code, "schema"], response: &response)
      }
    }
  }

  func checkSchemaType(_ schema:Openapi_V2_Schema,
                       path:[String],
                       response:inout Gnostic_Plugin_V1_Response) {
    if schema.hasType {
      for type in schema.type.value {
        if type == "array" {
          response.message(
            level: .error,
            code: "NO_ARRAY_RESPONSES",
            text: "Arrays MUST NOT be returned as the top-level structure in a response body.",
            path: path)
        }
      }
    }
  }

  func resolveReference(_ reference:String) -> Openapi_V2_Schema? {
    let prefix = "#/definitions/"
    if reference.hasPrefix(prefix) {
      let schemaName = reference.dropFirst(prefix.count)
      for pair in document.definitions.additionalProperties {
        if pair.name == schemaName {
          return pair.value
        }
      }
    }
    return nil
  }
}

func main() throws {
  let request = try Gnostic_Plugin_V1_Request(serializedData: Stdin.readall())
  var response = Gnostic_Plugin_V1_Response()
  try ResponseLinter().run(request, &response)
  let serializedResponse = try response.serializedData()
  Stdout.write(bytes: serializedResponse)
}

try main()
