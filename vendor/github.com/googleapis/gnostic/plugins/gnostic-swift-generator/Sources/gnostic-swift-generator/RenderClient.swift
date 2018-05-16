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

extension ServiceRenderer {
    
    func renderClient() -> String {
        var code = CodePrinter()
        code.print(header)
        code.print()
        code.print("// Client code")
        code.print()
        code.print("import Foundation")
        code.print("import Dispatch")
        code.print()
        code.print("""
enum ClientError: Swift.Error {
  case errorWithCode(Int)
}
""")
        code.print()
        code.print("public class Client {")
        code.indent()
        code.print("var service : String")
        code.print()
        code.print("""
public init(service: String) {
  self.service = service
}
""")
        for serviceMethod in self.methods {
            code.print()
            code.print("// " + serviceMethod.description + " Asynchronous.")
            code.print("public func " + serviceMethod.name + "(" + asyncClientParametersDeclaration(serviceMethod) + ") throws {")
            code.indent()
            code.print("var path = self.service")
            code.print("path = path + \"" + serviceMethod.path + "\"")
            for serviceTypeField in parametersTypeFields(serviceMethod) {
                if serviceTypeField.position == "path" {
                    code.print("path = path.replacingOccurrences(of:\"{" +
                        serviceTypeField.name +
                        "}\", with:\"\\(" +
                        serviceTypeField.name +
                        ")\")")
                }
            }
            code.print("guard let url = URL(string:path) else {")
            code.indent()
            code.print("throw ClientError.errorWithCode(0)")
            code.outdent()
            code.print("}")
            code.print("var request = URLRequest(url:url)")
            code.print("request.httpMethod = \"" + serviceMethod.method + "\"")
            for serviceTypeField in parametersTypeFields(serviceMethod) {
                if serviceTypeField.position == "body" {
                    code.print("let jsonObject = " + serviceTypeField.name + ".jsonObject()")
                    code.print("request.httpBody = try JSONSerialization.data(withJSONObject:jsonObject)")
                }
            }
            if hasResponses(serviceMethod) {
                code.print("fetch(request) {(data, response, error) in")
                code.indent()
                code.print("if error != nil {")
                code.indent()
                code.print("callback(nil, ClientError.errorWithCode(0))")
                code.print("return")
                code.outdent()
                code.print("}")
                code.print("guard let httpResponse = response else {")
                code.indent()
                code.print("callback(nil, ClientError.errorWithCode(0))")
                code.print("return")
                code.outdent()
                code.print("}")
                code.print("if httpResponse.statusCode == 200 {")
                code.indent()
                code.print("if let data = data {")
                code.indent()
                code.print("let jsonObject = try! JSONSerialization.jsonObject(with:data)")
                code.print("if let value = " + serviceMethod.resultTypeName! + "(jsonObject:jsonObject) {")
                code.indent()
                code.print("callback(value, nil)")
                code.print("return")
                code.outdent()
                code.print("}")
                code.outdent()
                code.print("}")
                code.print("callback(nil, nil)")
                code.outdent()
                code.print("} else {")
                code.indent()
                code.print(" callback(nil, ClientError.errorWithCode(httpResponse.statusCode))")
                code.outdent()
                code.print("}")
                code.outdent()
                code.print("}")
            } else {
                code.print("fetch(request) {(data, response, error) in")
                code.print("if error != nil {")
                code.indent()
                code.print("callback(ClientError.errorWithCode(0))")
                code.print("return")
                code.outdent()
                code.print("}")
                code.print("guard let httpResponse = response else {")
                code.indent()
                code.print("callback(ClientError.errorWithCode(0))")
                code.print("return")
                code.outdent()
                code.print("}")
                code.print("if httpResponse.statusCode == 200 {")
                code.indent()
                code.print("callback(nil)")
                code.print("} else {")
                code.indent()
                code.print("callback(ClientError.errorWithCode(httpResponse.statusCode))")
                code.outdent()
                code.print("}")
                code.outdent()
                code.print("}")
            }
            code.outdent()
            code.print("}")
            code.print()
            code.print("// " + serviceMethod.description + " Synchronous.")
            code.print("public func " + serviceMethod.name + "(" + syncClientParametersDeclaration(serviceMethod) + ") throws " + syncClientReturnDeclaration(serviceMethod) + " {")
            code.indent()
            code.print("let sem = DispatchSemaphore(value: 0)")
            if hasResponses(serviceMethod) {
                code.print("var response : " + serviceMethod.resultTypeName! + "?")
            }
            code.print("var error : Swift.Error?")
            if hasResponses(serviceMethod) {
                code.print("try " + serviceMethod.name + "(" + parameterFieldNames(serviceMethod) + ") {r, e in")
                code.indent()
                code.print("response = r")
            } else {
                code.print("try " + serviceMethod.name + "(" + parameterFieldNames(serviceMethod) + ") {e in")
                code.indent()
            }
            code.print("error = e")
            code.print("sem.signal()")
            code.outdent()
            code.print("}")
            code.print("sem.wait()")
            code.print("if let actualError = error {")
            code.indent()
            code.print("throw actualError")
            code.outdent()
            code.print("}")
            if hasResponses(serviceMethod) {
                code.print("if let actualResponse = response {")
                code.indent()
                code.print("return actualResponse")
                code.outdent()
                code.print("} else {")
                code.indent()
                code.print("throw ClientError.errorWithCode(0)")
                code.outdent()
                code.print("}")
            }
            code.outdent()
            code.print("}")
            code.print()
        }
        code.outdent()
        code.print("}")
        return code.content
    }
}

