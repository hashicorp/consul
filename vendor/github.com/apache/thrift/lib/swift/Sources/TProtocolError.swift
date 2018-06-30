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

import Foundation

public struct TProtocolError : TError {
  public init() { }

  public enum Code : TErrorCode {
    case unknown
    case invalidData
    case negativeSize
    case sizeLimit(limit: Int, got: Int)
    case badVersion(expected: String, got: String)
    case notImplemented
    case depthLimit

    public var thriftErrorCode: Int {
      switch self {
      case .unknown:        return 0
      case .invalidData:    return 1
      case .negativeSize:   return 2
      case .sizeLimit:      return 3
      case .badVersion:     return 4
      case .notImplemented: return 5
      case .depthLimit:     return 6
      }

    }
    public var description: String {
      switch self {
      case .unknown:        return "Unknown TProtocolError"
      case .invalidData:    return "Invalid Data"
      case .negativeSize:   return "Negative Size"
      case .sizeLimit(let limit, let got):
        return "Message exceeds size limit of \(limit) (received: \(got)"
      case .badVersion(let expected, let got):
        return "Bad Version. (Expected: \(expected), Got: \(got)"
      case .notImplemented: return "Not Implemented"
      case .depthLimit:     return "Depth Limit"
      }
    }
  }

  public enum ExtendedErrorCode : TErrorCode {
    case unknown
    case missingRequiredField(fieldName: String)
    case unexpectedType(type: TType)
    case mismatchedProtocol(expected: String, got: String)
    public var thriftErrorCode: Int {
      switch self {
      case .unknown:              return 1000
      case .missingRequiredField: return 1001
      case .unexpectedType:       return 1002
      case .mismatchedProtocol:   return 1003
      }
    }
    public var description: String {
      switch self {
      case .unknown:                                    return "Unknown TProtocolExtendedError"
      case .missingRequiredField(let fieldName):        return "Missing Required Field: \(fieldName)"
      case .unexpectedType(let type):                   return "Unexpected Type \(type.self)"
      case .mismatchedProtocol(let expected, let got):  return "Mismatched Protocol.  (Expected: \(expected), got \(got))"
      }
    }
  }

  public var extendedError: ExtendedErrorCode? = nil

  public init(error: Code = .unknown,
              message: String? = nil,
              extendedError: ExtendedErrorCode? = nil) {
    self.error = error
    self.message = message
    self.extendedError = extendedError
  }

  /// Mark: TError
  public var error: Code = .unknown
  public var message: String? = nil
  public static var defaultCase: Code { return .unknown }

  public var description: String {
    var out = "\(TProtocolError.self):  (\(error.thriftErrorCode) \(error.description)\n"
    if let extendedError = extendedError {
      out += "TProtocolExtendedError (\(extendedError.thriftErrorCode)): \(extendedError.description)"
    }
    if let message = message {
      out += "Message: \(message)"
    }
    return out
  }
}


/// Wrapper for Transport errors in Protocols.  Inspired by Thrift-Cocoa PROTOCOL_TRANSPORT_ERROR
/// macro.  Modified to be more Swift-y.  Catches any TError thrown within the block and
/// rethrows a given TProtocolError, the original error's description is appended to the new
/// TProtocolError's message.  sourceFile, sourceLine, sourceMethod are auto-populated and should
/// be ignored when calling.
///
/// - parameter error:        TProtocolError to throw if the block throws
/// - parameter sourceFile:   throwing file, autopopulated
/// - parameter sourceLine:   throwing line, autopopulated
/// - parameter sourceMethod: throwing method, autopopulated
/// - parameter block:        throwing block
///
/// - throws: TProtocolError  Default is TProtocolError.ErrorCode.unknown.  Underlying
///                           error's description appended to TProtocolError.message
func ProtocolTransportTry(error: TProtocolError = TProtocolError(),
                          sourceFile: String = #file,
                          sourceLine: Int = #line,
                          sourceMethod: String = #function,
                          block: () throws -> ()) throws {
  // Need mutable copy
  var error = error
  do {
    try block()
  } catch let err as TError {
    var message = error.message ?? ""
    message += "\nFile: \(sourceFile)\n"
    message += "Line: \(sourceLine)\n"
    message += "Method: \(sourceMethod)"
    message += "\nOriginal Error:\n" + err.description
    error.message = message
    throw error
  }
}


