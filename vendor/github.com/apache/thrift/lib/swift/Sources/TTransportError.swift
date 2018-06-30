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

public struct TTransportError: TError {
  public enum ErrorCode: TErrorCode {
    case unknown
    case notOpen
    case alreadyOpen
    case timedOut
    case endOfFile
    case negativeSize
    case sizeLimit(limit: Int, got: Int)
    
    public var thriftErrorCode: Int {
      switch self {
      case .unknown:      return 0
      case .notOpen:      return 1
      case .alreadyOpen:  return 2
      case .timedOut:     return 3
      case .endOfFile:    return 4
      case .negativeSize: return 5
      case .sizeLimit:    return 6
      }
    }
    public var description: String {
      switch self {
      case .unknown:      return "Unknown TTransportError"
      case .notOpen:      return "Not Open"
      case .alreadyOpen:  return "Already Open"
      case .timedOut:     return "Timed Out"
      case .endOfFile:    return "End Of File"
      case .negativeSize: return "Negative Size"
      case .sizeLimit(let limit, let got):
        return "Message exceeds size limit of \(limit) (received: \(got)"
      }
    }
  }
  public var error: ErrorCode = .unknown
  public var message: String? = nil
  public static var defaultCase: ErrorCode { return .unknown }
  
  public init() { }

}

/// THTTPTransportError
///
/// Error's thrown on HTTP Transport
public struct THTTPTransportError: TError {
  public enum ErrorCode: TErrorCode {
    case invalidResponse
    case invalidStatus(statusCode: Int)
    case authentication
    
    public var description: String {
      switch self {
      case .invalidResponse:                return "Invalid HTTP Response"
      case .invalidStatus(let statusCode):  return "Invalid HTTP Status Code (\(statusCode))"
      case .authentication:                 return "Authentication Error"
      }
    }
    public var thriftErrorCode: Int { return 0 }
  }
  public var error: ErrorCode = .invalidResponse
  public var message: String? = nil
  public static var defaultCase: ErrorCode { return .invalidResponse }
  
  public init() { }
}

