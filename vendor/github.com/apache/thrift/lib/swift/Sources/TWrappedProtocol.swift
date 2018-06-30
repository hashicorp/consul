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

import Foundation // For (NS)Data


/// Generic protocol, implementes TProtocol and wraps a concrete protocol.
/// Useful for generically subclassing protocols to override specific methods 
/// (i.e. TMultiplexedProtocol)
open class TWrappedProtocol<Protocol: TProtocol> : TProtocol {
  var concreteProtocol: Protocol
  
  public var transport: TTransport {
    get {
      return concreteProtocol.transport
    }
    set {
      concreteProtocol.transport = newValue
    }
  }

  public required init(on transport: TTransport) {
    self.concreteProtocol = Protocol(on: transport)
  }
  
  // Read methods
  
  public func readMessageBegin() throws -> (String, TMessageType, Int32) {
    return try concreteProtocol.readMessageBegin()
  }
  
  public func readMessageEnd() throws {
    try concreteProtocol.readMessageEnd()
  }
  
  public func readStructBegin() throws -> String {
    return try concreteProtocol.readStructBegin()
  }
  
  public func readStructEnd() throws {
    try concreteProtocol.readStructEnd()
  }
  
  public func readFieldBegin() throws -> (String, TType, Int32) {
    return try concreteProtocol.readFieldBegin()
  }
  
  public func readFieldEnd() throws {
    try concreteProtocol.readFieldEnd()
  }
  
  public func readMapBegin() throws -> (TType, TType, Int32) {
    return try concreteProtocol.readMapBegin()
  }
  
  public func readMapEnd() throws {
    try concreteProtocol.readMapEnd()
  }
  
  public func readSetBegin() throws -> (TType, Int32) {
    return try concreteProtocol.readSetBegin()
  }
  
  public func readSetEnd() throws {
    try concreteProtocol.readSetEnd()
  }
  
  public func readListBegin() throws -> (TType, Int32) {
    return try concreteProtocol.readListBegin()
  }
  
  public func readListEnd() throws {
    try concreteProtocol.readListEnd()
  }
  
  public func read() throws -> String {
    return try concreteProtocol.read()
  }
  
  public func read() throws -> Bool {
    return try concreteProtocol.read()
  }
  
  public func read() throws -> UInt8 {
    return try concreteProtocol.read()
  }
  
  public func read() throws -> Int16 {
    return try concreteProtocol.read()
  }
  
  public func read() throws -> Int32 {
    return try concreteProtocol.read()
  }
  
  public func read() throws -> Int64 {
    return try concreteProtocol.read()
  }
  
  public func read() throws -> Double {
    return try concreteProtocol.read()
  }
  
  public func read() throws -> Data {
    return try concreteProtocol.read()
  }
  
  // Write methods
  
  public func writeMessageBegin(name: String, type messageType: TMessageType, sequenceID: Int32) throws {
    return try concreteProtocol.writeMessageBegin(name: name, type: messageType, sequenceID: sequenceID)
  }
  
  public func writeMessageEnd() throws {
    try concreteProtocol.writeMessageEnd()
  }
  
  public func writeStructBegin(name: String) throws {
    try concreteProtocol.writeStructBegin(name: name)
  }
  
  public func writeStructEnd() throws {
    try concreteProtocol.writeStructEnd()
  }
  
  public func writeFieldBegin(name: String, type fieldType: TType, fieldID: Int32) throws {
    try concreteProtocol.writeFieldBegin(name: name, type: fieldType, fieldID: fieldID)
  }
  
  public func writeFieldStop() throws {
    try concreteProtocol.writeFieldStop()
  }
  
  public func writeFieldEnd() throws {
    try concreteProtocol.writeFieldEnd()
  }
  
  public func writeMapBegin(keyType: TType, valueType: TType, size: Int32) throws {
    try concreteProtocol.writeMapBegin(keyType: keyType, valueType: valueType, size: size)
  }
  
  public func writeMapEnd() throws {
    try concreteProtocol.writeMapEnd()
  }
  
  public func writeSetBegin(elementType: TType, size: Int32) throws {
    try concreteProtocol.writeSetBegin(elementType: elementType, size: size)
  }
  
  public func writeSetEnd() throws {
    try concreteProtocol.writeSetEnd()
  }
  
  public func writeListBegin(elementType: TType, size: Int32) throws {
    try concreteProtocol.writeListBegin(elementType: elementType, size: size)
  }
  
  public func writeListEnd() throws {
    try concreteProtocol.writeListEnd()
  }
  public func write(_ value: String) throws {
    try concreteProtocol.write(value)
  }
  
  public func write(_ value: Bool) throws {
    try concreteProtocol.write(value)
  }
  
  public func write(_ value: UInt8) throws {
    try concreteProtocol.write(value)
  }

  public func write(_ value: Int16) throws {
    try concreteProtocol.write(value)
  }
  
  public func write(_ value: Int32) throws {
    try concreteProtocol.write(value)
  }
  
  public func write(_ value: Int64) throws {
    try concreteProtocol.write(value)
  }
  
  public func write(_ value: Double) throws {
    try concreteProtocol.write(value)
  }

  public func write(_ data: Data) throws {
    try concreteProtocol.write(data)
  }
}
