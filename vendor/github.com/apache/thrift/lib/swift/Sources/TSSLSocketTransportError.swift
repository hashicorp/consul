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

public struct TSSLSocketTransportError: TError {
  public enum ErrorCode: TErrorCode {
    case hostanameResolution(hostname: String)
    case socketCreate(port: Int)
    case connect
  
    public var thriftErrorCode: Int {
      switch self {
      case .hostanameResolution:  return -10000
      case .socketCreate:         return -10001
      case .connect:              return -10002
      }
    }
  
    public var description: String {
      switch self {
      case .hostanameResolution(let hostname):  return "Failed to resolve hostname: \(hostname)"
      case .socketCreate(let port):             return "Could not create socket on port: \(port)"
      case .connect:                            return "Connect error"
      }
    }
  
  }
  public var error: ErrorCode = .connect
  public var message: String?
  public static var defaultCase: ErrorCode { return .connect }
  
  public init() { }
}
