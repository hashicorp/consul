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
    
    func renderTypes() -> String {
        var code = CodePrinter()
        code.print(header)
        code.print()
        code.print("// Common type declarations")
        for serviceType in self.types {
            code.print()
            code.print("public class " + serviceType.name + " : CustomStringConvertible {")
            code.indent()
            for serviceTypeField in serviceType.fields {
                code.print("public var " + serviceTypeField.name + " : " + serviceTypeField.typeName + " = " + serviceTypeField.initialValue)
            }
            code.print()
            code.print("public init() {}")
            code.print()
            if serviceType.isInterfaceType {
                code.print("public init?(jsonObject: Any?) {")
                code.indent()
                code.print("if let jsonDictionary = jsonObject as? [String:Any] {")
                code.indent()
                for serviceTypeField in serviceType.fields {
                    code.print("if let value : Any = jsonDictionary[\"" + serviceTypeField.jsonName + "\"] {")
                    code.indent()
                    if serviceTypeField.isArrayType {
                        code.print("var outArray : [" + serviceTypeField.elementType + "] = []")
                        code.print("let array = value as! [Any]")
                        code.print("for arrayValue in array {")
                        code.indent()
                        code.print("if let element = " + serviceTypeField.elementType + "(jsonObject:arrayValue) {")
                        code.indent()
                        code.print("outArray.append(element)")
                        code.outdent()
                        code.print("}")
                        code.outdent()
                        code.print("}")
                        code.print("self." + serviceTypeField.name + " = outArray")
                    } else if serviceTypeField.isCastableType {
                        code.print("self." + serviceTypeField.name + " = value as! " + serviceTypeField.typeName)
                    } else if serviceTypeField.isConvertibleType {
                        code.print("self." + serviceTypeField.name + " = " + serviceTypeField.typeName + "(value)")
                    }
                    code.outdent()
                    code.print("}")
                }
                code.outdent()
                code.print("} else {")
                code.indent()
                code.print("return nil")
                code.outdent()
                code.print("}")
                code.outdent()
                code.print("}")
                code.print()
                code.print("public func jsonObject() -> Any {")
                code.indent()
                code.print("var result : [String:Any] = [:]")
                for serviceTypeField in serviceType.fields {
                    if serviceTypeField.isArrayType {
                        code.print("var outArray : [Any] = []")
                        code.print("for arrayValue in self." + serviceTypeField.name + " {")
                        code.indent()
                        code.print("outArray.append(arrayValue.jsonObject())")
                        code.outdent()
                        code.print("}")
                        code.print("result[\"" + serviceTypeField.jsonName + "\"] = outArray")
                    }
                    if serviceTypeField.isCastableType {
                        code.print("result[\"" + serviceTypeField.jsonName + "\"] = self." + serviceTypeField.name)
                    }
                    if serviceTypeField.isConvertibleType {
                        code.print("result[\"" + serviceTypeField.jsonName + "\"] = self." + serviceTypeField.name + ".jsonObject()")
                    }
                }
                code.print("return result")
                code.outdent()
                code.print("}")
                code.print()
            }
            code.print("public var description : String {")
            code.indent()
            code.print("return \"[" + serviceType.name + "\" + ")
            for serviceTypeField in serviceType.fields {
                code.print("  \" " + serviceTypeField.name + ": \" + String(describing:self." + serviceTypeField.name + ") + ")
            }
            code.print("\"]\"")
            code.outdent()
            code.print("}")
            code.outdent()
            code.print("}")
        }
        return code.content
    }
}
