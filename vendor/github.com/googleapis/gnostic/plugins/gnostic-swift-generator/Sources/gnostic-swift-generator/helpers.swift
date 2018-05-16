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

func hasParameters(_ value : Any?) -> Bool {
    let method : ServiceMethod = value as! ServiceMethod
    return method.parametersType != nil
}

func hasResponses(_ value : Any?) -> Bool {
    let method : ServiceMethod = value as! ServiceMethod
    return method.responsesType != nil
}

func syncClientParametersDeclaration(_ value: Any?) -> String {
    let method : ServiceMethod = value as! ServiceMethod
    var result = ""
    if let parametersType = method.parametersType {
        for field in parametersType.fields {
            if result != "" {
                result += ", "
            }
            result += field.name + " : " + field.typeName
        }
    }
    return result
}

func syncClientReturnDeclaration(_ value : Any?) -> String {
    let method : ServiceMethod = value as! ServiceMethod
    var result = ""
    if let resultTypeName = method.resultTypeName {
        result = " -> " + resultTypeName
    }
    return result
}

func asyncClientParametersDeclaration(_ value : Any?) -> String {
    let method : ServiceMethod = value as! ServiceMethod
    var result = ""
    if let parametersType = method.parametersType {
        for field in parametersType.fields {
            if result != "" {
                result += ", "
            }
            result += field.name + " : " + field.typeName
        }
    }
    // add callback
    if result != "" {
        result += ", "
    }
    if let resultTypeName = method.resultTypeName {
        result += "callback : @escaping (" + resultTypeName + "?, Swift.Error?)->()"
    } else {
        result += "callback : @escaping (Swift.Error?)->()"
    }
    return result
}

func protocolParametersDeclaration(_ value: Any?) -> String {
    let method : ServiceMethod = value as! ServiceMethod
    var result = ""
    if let parametersTypeName = method.parametersTypeName {
        result = "_ parameters : " + parametersTypeName
    }
    return result
}

func protocolReturnDeclaration(_ value: Any?) -> String {
    let method : ServiceMethod = value as! ServiceMethod
    var result = ""
    if let responsesTypeName = method.responsesTypeName {
        result = "-> " + responsesTypeName
    }
    return result
}

func parameterFieldNames(_ value: Any?) -> String {
    let method : ServiceMethod = value as! ServiceMethod
    var result = ""
    if let parametersType = method.parametersType {
        for field in parametersType.fields {
            if result != "" {
                result += ", "
            }
            result += field.name + ":" + field.name
        }
    }
    return result
}

func parametersTypeFields(_ value: Any?) -> [ServiceTypeField] {
    let method : ServiceMethod = value as! ServiceMethod
    if let parametersType = method.parametersType {
        return parametersType.fields
    } else {
        return []
    }
}

func kituraPath(_ value: Any?) -> String {
    let method : ServiceMethod = value as! ServiceMethod
    var path = method.path
    if let parametersType = method.parametersType {
        for field in parametersType.fields {
            if field.position == "path" {
                let original = "{" + field.jsonName + "}"
                let replacement = ":" + field.jsonName
                path = path.replacingOccurrences(of:original, with:replacement)
            }
        }
    }
    return path
}

func bodyParameterFieldName(_ value: Any?) -> String {
    let method : ServiceMethod = value as! ServiceMethod
    if let parametersType = method.parametersType {
        for field in parametersType.fields {
            if field.position == "body" {
                return field.name
            }
        }
    }
    return ""
}

func responsesHasFieldNamedOK(_ value: Any?) -> Bool {
    let method : ServiceMethod = value as! ServiceMethod
    if let responsesType = method.responsesType {
        for field in responsesType.fields {
            if field.name == "ok" {
                return true
            }
        }
    }
    return false
}

func responsesHasFieldNamedError(_ value: Any?) -> Bool {
    let method : ServiceMethod = value as! ServiceMethod
    if let responsesType = method.responsesType {
        for field in responsesType.fields {
            if field.name == "error" {
                return true
            }
        }
    }
    return false
}

func lowercase(_ s : String) -> String {
    return s.lowercased()
}

