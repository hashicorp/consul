/*
* Licensed to the Apache Software Foundation (ASF) under one
* or more contributor license agreements. See the NOTICE file
* distributed with this work for additional information
* regarding copyright ownership. The ASF licenses this file
* to you under the Apache License, Version 2.0 (the
* "License"); you may not use this file except in compliance
* with the License. You may obtain a copy of the License at
*
*   http://www.apache.org/licenses/LICENSE-2.0
*
* Unless required by applicable law or agreed to in writing,
* software distributed under the License is distributed on an
* "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
* KIND, either express or implied. See the License for the
* specific language governing permissions and limitations
* under the License.
*/


public struct TApplicationError : TError {
  public enum Code : TErrorCode {
    case unknown
    case unknownMethod(methodName: String?)
    case invalidMessageType
    case wrongMethodName(methodName: String?)
    case badSequenceId
    case missingResult(methodName: String?)
    case internalError
    case protocolError
    case invalidTransform
    case invalidProtocol
    case unsupportedClientType
    
    
    /// Initialize a TApplicationError with a Thrift error code
    /// Normally this would be achieved with RawRepresentable however
    /// by doing this we can allow for associated properties on enum cases for
    /// case specific context data in a Swifty, type-safe manner.
    ///
    /// - parameter thriftErrorCode: Integer TApplicationError(exception) error code.  
    ///                              Default to 0 (.unknown)
    public init(thriftErrorCode: Int) {
      switch thriftErrorCode {
      case 1:  self = .unknownMethod(methodName: nil)
      case 2:  self = .invalidMessageType
      case 3:  self = .wrongMethodName(methodName: nil)
      case 4:  self = .badSequenceId
      case 5:  self = .missingResult(methodName: nil)
      case 6:  self = .internalError
      case 7:  self = .protocolError
      case 8:  self = .invalidProtocol
      case 9:  self = .invalidTransform
      case 10: self = .unsupportedClientType
      default: self = .unknown
      }
    }
    public var thriftErrorCode: Int {
      switch self {
      case .unknown:                return 0
      case .unknownMethod:          return 1
      case .invalidMessageType:     return 2
      case .wrongMethodName:        return 3
      case .badSequenceId:          return 4
      case .missingResult:          return 5
      case .internalError:          return 6
      case .protocolError:          return 7
      case .invalidProtocol:        return 8
      case .invalidTransform:       return 9
      case .unsupportedClientType:  return 10
      }
    }
    
    public var description: String {
      /// Output "for #methodName" if method is not nil else empty
      let methodUnwrap: (String?) -> String =  { method in
        return "\(method == nil ? "" : " for \(method ?? "")")"
      }
      switch self {
      case .unknown:                      return "Unknown TApplicationError"
      case .unknownMethod(let method):    return "Unknown Method\(methodUnwrap(method))"
      case .invalidMessageType:           return "Invalid Message Type"
      case .wrongMethodName(let method):  return "Wrong Method Name\(methodUnwrap(method))"
      case .badSequenceId:                return "Bad Sequence ID"
      case .missingResult(let method):    return "Missing Result\(methodUnwrap(method))"
      case .internalError:                return "Internal Error"
      case .protocolError:                return "Protocol Error"
      case .invalidProtocol:              return "Invalid Protocol"
      case .invalidTransform:             return "Invalid Transform"
      case .unsupportedClientType:        return "Unsupported Client Type"
      }
    }
  }

  public init() { }
  
  public init(thriftErrorCode code: Int, message: String? = nil) {
    self.error = Code(thriftErrorCode: code)
    self.message = message
  }
  
  public var error: Code = .unknown
  public var message: String? = nil
  public static var defaultCase: Code { return .unknown }
}

extension TApplicationError : TSerializable {
  public static var thriftType: TType { return .struct }
  
  public static func read(from proto: TProtocol) throws -> TApplicationError {
    var errorCode: Int = 0
    var message: String? = nil
    _ = try proto.readStructBegin()
    fields: while true {
      let (_, fieldType, fieldID) = try proto.readFieldBegin()
      
      switch (fieldID, fieldType) {
      case (_, .stop):
        break fields
      case (1, .string):
        message = try proto.read()
      case (2, .i32):
        errorCode = Int(try proto.read() as Int32)
      
      case let (_, unknownType):
        try proto.skip(type: unknownType)
      }
      
      try proto.readFieldEnd()
    }
    try proto.readStructEnd()
    return TApplicationError(thriftErrorCode: errorCode, message: message)
  }

  public func write(to proto: TProtocol) throws {
    try proto.writeStructBegin(name: "TApplicationException")
    
    try proto.writeFieldBegin(name: "message", type: .string, fieldID: 1)
    try proto.write(message ?? "")
    try proto.writeFieldEnd()
    
    try proto.writeFieldBegin(name: "type", type: .i32, fieldID: 2)
    let val = Int32(error.thriftErrorCode)
    try proto.write(val)
    try proto.writeFieldEnd()
    try proto.writeFieldStop()
    try proto.writeStructEnd()
  }
  
  public var hashValue: Int {
    return error.thriftErrorCode &+ (message?.hashValue ?? 0)
  }
}

public func ==(lhs: TApplicationError, rhs: TApplicationError) -> Bool {
  return lhs.error.thriftErrorCode == rhs.error.thriftErrorCode && lhs.message == rhs.message
}
