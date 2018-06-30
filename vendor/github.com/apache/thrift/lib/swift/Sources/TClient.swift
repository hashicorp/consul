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


open class TClient {
  public let inProtocol: TProtocol
  public let outProtocol: TProtocol

  public init(inoutProtocol: TProtocol) {
    self.inProtocol = inoutProtocol
    self.outProtocol = inoutProtocol
  }

  public init(inProtocol: TProtocol, outProtocol: TProtocol) {
    self.inProtocol = inProtocol
    self.outProtocol = outProtocol
  }
}


open class TAsyncClient<Protocol: TProtocol, Factory: TAsyncTransportFactory> {
  public var factory: Factory
  public init(with protocol: Protocol.Type, factory: Factory) {
    self.factory = factory
  }
}


public enum TAsyncResult<T> {
  case success(T)
  case error(Swift.Error)
  
  public func value() throws -> T {
    switch self {
    case .success(let t): return t
    case .error(let e): throw e
    }
  }
}
