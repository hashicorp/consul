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
    
    func renderServer() -> String {
        var code = CodePrinter()
        code.print(header)
        code.print()
        code.print("// Service code")
        code.print("import Kitura")
        code.print("import KituraNet")
        code.print("import Foundation")
        code.print("// A server requires an instance of an implementation of this protocol.")
        code.print("public protocol Service {")
        code.indent()
        for serviceMethod in self.methods {
            code.print("// " + serviceMethod.description)
            code.print("func " + serviceMethod.name + " (" +
                protocolParametersDeclaration(serviceMethod) + ") throws " +
                protocolReturnDeclaration(serviceMethod))
        }
        code.outdent()
        code.print("}")
        
        code.print("func intValue(_ s:String?) -> Int64 {")
        code.indent()
        code.print("guard let s = s else {")
        code.indent()
        code.print("return 0")
        code.outdent()
        code.print("}")
        code.print("guard let value = Int64(s) else {")
        code.indent()
        code.print("return 0")
        code.outdent()
        code.print("}")
        code.print("return value")
        code.outdent()
        code.print("}")
        code.print("public func server(service : Service) -> Router {")
        code.indent()
        code.print("// Create a new router")
        code.print("let router = Router()")
        for serviceMethod in self.methods {
            code.print("// " + serviceMethod.description)
            code.print("router." + lowercase(serviceMethod.method) + "(\"" + kituraPath(serviceMethod) + "\") { req, res, next in")
            code.indent()
            if hasParameters(serviceMethod) {
                code.print("// instantiate the parameters structure")
                code.print("let parameters = " + serviceMethod.parametersTypeName! + "()")
                for serviceTypeField in parametersTypeFields(serviceMethod) {
                    if serviceTypeField.position == "path" {
                        code.print("parameters." + serviceTypeField.name +
                            " = intValue(req.parameters[\"" +
                            serviceTypeField.name + "\"])")
                    }
                }
                if serviceMethod.method == "POST" {
                    code.print("// deserialize request from post data")
                    code.print("let bodyString = try req.readString() ?? \"\"")
                    code.print("guard let bodyData = bodyString.data(using:.utf8) else {")
                    code.indent()
                    code.print("try res.send(status:.badRequest).end()")
                    code.print("return")
                    code.outdent()
                    code.print("}")
                    code.print("var jsonObject : Any? = nil")
                    code.print("do {")
                    code.indent()
                    code.print("jsonObject = try JSONSerialization.jsonObject(with:bodyData)")
                    code.outdent()
                    code.print("} catch {")
                    code.indent()
                    code.print("try res.send(status:.badRequest).end()")
                    code.print("return")
                    code.outdent()
                    code.print("}")
                    code.print("guard let bodyObject = " + serviceMethod.resultTypeName! + "(jsonObject:jsonObject) else {")
                    code.print("try res.send(status:.badRequest).end()")
                    code.indent()
                    code.print("return")
                    code.outdent()
                    code.print("}")
                    code.print("parameters." + bodyParameterFieldName(serviceMethod) + " = bodyObject")
                }
            }
            if hasParameters(serviceMethod) {
                if hasResponses(serviceMethod) {
                    code.print("let responses = try service." + serviceMethod.name + "(parameters)")
                } else {
                    code.print("try service." + serviceMethod.name + "(parameters)")
                }
            } else {
                if hasResponses(serviceMethod) {
                    code.print("let responses = try service." + serviceMethod.name + "()")
                } else {
                    code.print("try service." + serviceMethod.name + "()")
                }
            }
            if hasResponses(serviceMethod) {
                if responsesHasFieldNamedOK(serviceMethod) {
                    code.print("if let ok = responses.ok {")
                    code.indent()
                    code.print("let jsonObject = ok.jsonObject()")
                    code.print("let responseData = try JSONSerialization.data(withJSONObject:jsonObject)")
                    code.print("try res.send(data:responseData).end()")
                    code.print("return")
                    code.outdent()
                    code.print("}")
                }
                if responsesHasFieldNamedError(serviceMethod) {
                    code.print("if let errorResponse = responses.error {")
                    code.indent()
                    code.print("guard let statusCode = HTTPStatusCode(rawValue:Int(errorResponse.code)) else {")
                    code.indent()
                    code.print("try res.send(status:.unknown).end()")
                    code.print("return")
                    code.outdent()
                    code.print("}")
                    code.print("try res.send(status:statusCode).end()")
                    code.print("return")
                    code.outdent()
                    code.print("}")
                }
                code.print("try res.send(status:.internalServerError).end()")
            } else {
                code.print("try res.send(status:.OK).end()")
            }
            code.outdent()
            code.print("}")
        }
        code.print("return router")
        code.outdent()
        code.print("}")
        code.print("public func initialize(service: Service, port:Int) {")
        code.indent()
        code.print("// Create a new router")
        code.print("let router = server(service:service)")
        code.print("// Add an HTTP server and connect it to the router")
        code.print("Kitura.addHTTPServer(onPort:port, with: router)")
        code.outdent()
        code.print("}")
        code.print("public func run() {")
        code.indent()
        code.print("// Start the Kitura runloop (this call never returns)")
        code.print("Kitura.run()")
        code.outdent()
        code.print("}")
        return code.content
    }
}
