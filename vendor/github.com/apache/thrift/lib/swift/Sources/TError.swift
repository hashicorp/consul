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


/// TErrorCode
///
/// Protocol for TError conformers' enum's to conform to.
/// Generic Int Thrift error code to allow error cases to have
/// associated values.
public protocol TErrorCode : CustomStringConvertible {
  var thriftErrorCode: Int { get }
}

/// TError
///
/// Base protocol for all Thrift Error(Exception) types to conform to
public protocol TError : Error, CustomStringConvertible {

  /// Enum for error cases.  Can be typealiased to any conforming enum
  /// or defined nested.
  associatedtype Code: TErrorCode
  
  /// Error Case, value from internal enum
  var error: Code { get set }
  
  /// Optional additional message
  var message: String? { get set }
  
  /// Default error case for the error type, used for generic init()
  static var defaultCase: Code { get }
  
  init()
}

extension TError {
  /// Human readable description of error. Default provided for you in the
  /// format \(Self.self): \(error.errorDescription) \n message
  /// eg:
  ///
  ///     TApplicationError (1): Invalid Message Type
  ///     An unknown Error has occured.
  public var description: String {
    var out = "\(Self.self) (\(error.thriftErrorCode)): " + error.description + "\n"
    if let message = message {
      out += "Message: \(message)"
    }
    return out
  }

  /// Simple default Initializer for TError's
  ///
  /// - parameter error:   ErrorCode value.  Default: defaultCase
  /// - parameter message: Custom message with error.  Optional
  ///
  /// - returns: <#return value description#>
  public init(error: Code, message: String? = nil) {
    self.init()
    self.error = error
    self.message = message
  }
}
