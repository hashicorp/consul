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

public protocol TTransport {
  
  // Required
  func read(size: Int) throws -> Data
  func write(data: Data) throws
  func flush() throws

  // Optional (default provided)
  func readAll(size: Int) throws -> Data
  func isOpen() throws -> Bool
  func open() throws
  func close() throws
}

public extension TTransport {
  func isOpen() throws -> Bool { return true }
  func open() throws { }
  func close() throws { }
  
  func readAll(size: Int) throws -> Data {
    var buff = Data()
    var have = 0
    while have < size {
      let chunk = try self.read(size: size - have)
      have += chunk.count
      buff.append(chunk)
      if chunk.count == 0 {
        throw TTransportError(error: .endOfFile)
      }
    }
    return buff
  }
}

public protocol TAsyncTransport : TTransport {
  // Factory
  func flush(_ completion: @escaping (TAsyncTransport, Error?) ->())
}

public protocol TAsyncTransportFactory {
  associatedtype Transport : TAsyncTransport
  func newTransport() -> Transport
}
