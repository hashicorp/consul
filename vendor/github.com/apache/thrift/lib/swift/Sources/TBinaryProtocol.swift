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

public struct TBinaryProtocolVersion {
  static let version1    = Int32(bitPattern: 0x80010000)
  static let versionMask = Int32(bitPattern: 0xffff0000)
}

public class TBinaryProtocol: TProtocol {
  public var messageSizeLimit: UInt32  = 0
  
  public var transport: TTransport
  
  // class level properties for setting global config (useful for server in lieu of Factory design)
  public static var strictRead: Bool = false
  public static var strictWrite: Bool = true

  private var strictRead: Bool
  private var strictWrite: Bool
  
  var currentMessageName: String?
  var currentFieldName: String?
  
  
  public convenience init(transport: TTransport, strictRead: Bool, strictWrite: Bool) {
    self.init(on: transport)
    self.strictRead = strictRead
    self.strictWrite = strictWrite
  }
  
  public required init(on transport: TTransport) {
    self.transport = transport
    self.strictWrite = TBinaryProtocol.strictWrite
    self.strictRead = TBinaryProtocol.strictRead
  }
  
  func readStringBody(_ size: Int) throws -> String {
    
    var data = Data()
    try ProtocolTransportTry(error: TProtocolError(message: "Transport read failed")) {
      data = try self.transport.readAll(size: size)
    }
    
    return String(data: data, encoding: String.Encoding.utf8) ?? ""
  }
  
  /// Mark: - TProtocol
  
  public func readMessageBegin() throws -> (String, TMessageType, Int32) {
    let size: Int32 = try read()
    var messageName = ""
    var type = TMessageType.exception
    
    if size < 0 {
      let version = size & TBinaryProtocolVersion.versionMask
      if version != TBinaryProtocolVersion.version1 {
        throw TProtocolError(error: .badVersion(expected: "\(TBinaryProtocolVersion.version1)",
                                                got: "\(version)"))
      }
      type = TMessageType(rawValue: Int32(size) & 0x00FF) ?? type
      messageName = try read()
    } else {
      if strictRead {
        let errorMessage = "Missing message version, old client? Message Name: \(currentMessageName)"
        throw TProtocolError(error: .invalidData,
                             message: errorMessage)
      }
      if messageSizeLimit > 0 && size > Int32(messageSizeLimit) {
        throw TProtocolError(error: .sizeLimit(limit: Int(messageSizeLimit), got: Int(size)))
      }
      
      messageName = try readStringBody(Int(size))
      type = TMessageType(rawValue: Int32(try read() as UInt8)) ?? type
    }
    
    let seqID: Int32 = try read()
    return (messageName, type, seqID)
  }
  
  public func readMessageEnd() throws {
    return
  }
  
  public func readStructBegin() throws -> String {
    return ""
  }
  
  public func readStructEnd() throws {
    return
  }
  
  public func readFieldBegin() throws -> (String, TType, Int32) {
    
    let fieldType = TType(rawValue: Int32(try read() as UInt8)) ?? TType.stop
    var fieldID: Int32 = 0
    
    if fieldType != .stop {
      fieldID = Int32(try read() as Int16)
    }
    
    return ("", fieldType, fieldID)
  }
  
  public func readFieldEnd() throws {
    return
  }
  
  public func readMapBegin() throws -> (TType, TType, Int32) {
    var raw = Int32(try read() as UInt8)
    guard let keyType = TType(rawValue: raw) else {
      throw TProtocolError(message: "Unknown value for keyType TType: \(raw)")
    }
    
    raw = Int32(try read() as UInt8)
    guard let valueType = TType(rawValue: raw) else {
      throw TProtocolError(message: "Unknown value for valueType TType: \(raw)")
    }
    let size: Int32 = try read()
    
    return (keyType, valueType, size)
  }
  
  public func readMapEnd() throws {
    return
  }
  
  public func readSetBegin() throws -> (TType, Int32) {
    let raw = Int32(try read() as UInt8)
    guard let elementType = TType(rawValue: raw) else {
      throw TProtocolError(message: "Unknown value for elementType TType: \(raw)")
    }
    
    let size: Int32 = try read()
    
    return (elementType, size)
  }
  
  public func readSetEnd() throws {
    return
  }
  
  public func readListBegin() throws -> (TType, Int32) {
    let raw = Int32(try read() as UInt8)
    guard let elementType = TType(rawValue: raw) else {
      throw TProtocolError(message: "Unknown value for elementType TType: \(raw)")
    }
    let size: Int32 = try read()
    
    return (elementType, size)
  }
  
  public func readListEnd() throws {
    return
  }
  
  public func read() throws -> String {
    let data: Data = try read()
    guard let str = String.init(data: data, encoding: .utf8) else {
      throw TProtocolError(error: .invalidData, message: "Couldn't encode UTF-8 from data read")
    }
    return str
  }
  
  public func read() throws -> Bool {
    return (try read() as UInt8) == 1
  }
  
  public func read() throws -> UInt8 {
    var buff = Data()
    try ProtocolTransportTry(error: TProtocolError(message: "Transport Read Failed")) {
      buff = try self.transport.readAll(size: 1)
    }
    return buff[0]
  }
  
  public func read() throws -> Int16 {
    var buff = Data()
    try ProtocolTransportTry(error: TProtocolError(message: "Transport Read Failed")) {
      buff = try self.transport.readAll(size: 2)
    }
    var ret = Int16(buff[0] & 0xff) << 8
    ret |=    Int16(buff[1] & 0xff)
    return ret
  }
  
  public func read() throws -> Int32 {
    var buff = Data()
    try ProtocolTransportTry(error: TProtocolError(message: "Transport Read Failed")) {
      buff = try self.transport.readAll(size: 4)
    }
    var ret = Int32(buff[0] & 0xff) << 24
    ret |=    Int32(buff[1] & 0xff) << 16
    ret |=    Int32(buff[2] & 0xff) << 8
    ret |=    Int32(buff[3] & 0xff)
    
    return ret
  }
  
  public func read() throws -> Int64 {
    var buff = Data()
    try ProtocolTransportTry(error: TProtocolError(message: "Transport Read Failed")) {
      buff = try self.transport.readAll(size: 8)
    }
    var ret = Int64(buff[0] & 0xff) << 56
    ret |=    Int64(buff[1] & 0xff) << 48
    ret |=    Int64(buff[2] & 0xff) << 40
    ret |=    Int64(buff[3] & 0xff) << 32
    ret |=    Int64(buff[4] & 0xff) << 24
    ret |=    Int64(buff[5] & 0xff) << 16
    ret |=    Int64(buff[6] & 0xff) << 8
    ret |=    Int64(buff[7] & 0xff)
    
    return ret
  }
  
  public func read() throws -> Double {
    let val = try read() as Int64
    return unsafeBitCast(val, to: Double.self)
  }
  
  public func read() throws -> Data {
    let size = Int(try read() as Int32)
    var data = Data()
    try ProtocolTransportTry(error: TProtocolError(message: "Transport Read Failed")) {
      data = try self.transport.readAll(size: size)
    }
    
    return data
  }
  
  // Write methods
  
  public func writeMessageBegin(name: String, type messageType: TMessageType, sequenceID: Int32) throws {
    if strictWrite {
      let version = TBinaryProtocolVersion.version1 | Int32(messageType.rawValue)
      try write(version)
      try write(name)
      try write(sequenceID)
    } else {
      try write(name)
      try write(UInt8(messageType.rawValue))
      try write(sequenceID)
    }
    currentMessageName = name
  }
  
  public func writeMessageEnd() throws {
    currentMessageName = nil
  }
  
  public func writeStructBegin(name: String) throws {
    return
  }
  
  public func writeStructEnd() throws {
    return
  }
  
  public func writeFieldBegin(name: String, type fieldType: TType, fieldID: Int32) throws {
    try write(UInt8(fieldType.rawValue))
    try write(Int16(fieldID))
  }
  
  public func writeFieldStop() throws {
    try write(UInt8(TType.stop.rawValue))
  }
  
  public func writeFieldEnd() throws {
    return
  }
  
  public func writeMapBegin(keyType: TType, valueType: TType, size: Int32) throws {
    try write(UInt8(keyType.rawValue))
    try write(UInt8(valueType.rawValue))
    try write(size)
  }
  
  public func writeMapEnd() throws {
    return
  }
  
  public func writeSetBegin(elementType: TType, size: Int32) throws {
    try write(UInt8(elementType.rawValue))
    try write(size)
  }
  
  public func writeSetEnd() throws {
    return
  }
  
  public func writeListBegin(elementType: TType, size: Int32) throws {
    try write(UInt8(elementType.rawValue))
    try write(size)
  }
  
  public func writeListEnd() throws {
    return
  }
  
  public func write(_ value: String) throws {
    try write(value.data(using: .utf8)!)
  }
  
  public func write(_ value: Bool) throws {
    let byteVal: UInt8 = value ? 1 : 0
    try write(byteVal)
  }
  
  public func write(_ value: UInt8) throws {
    let buff = Data(bytes: [value])
    
    try ProtocolTransportTry(error: TProtocolError(message: "Transport write failed")) {
      try self.transport.write(data: buff)
    }
  }
  
  public func write(_ value: Int16) throws {
    var buff = Data()
    buff.append(Data(bytes: [UInt8(0xff & (value >> 8))]))
    buff.append(Data(bytes: [UInt8(0xff & (value))]))
    try ProtocolTransportTry(error: TProtocolError(message: "Transport write failed")) {
      try self.transport.write(data: buff)
    }
  }
  
  public func write(_ value: Int32) throws {
    var buff = Data()
    buff.append(Data(bytes: [UInt8(0xff & (value >> 24))]))
    buff.append(Data(bytes: [UInt8(0xff & (value >> 16))]))
    buff.append(Data(bytes: [UInt8(0xff & (value >> 8))]))
    buff.append(Data(bytes: [UInt8(0xff & (value))]))
    
    try ProtocolTransportTry(error: TProtocolError(message: "Transport write failed")) {
      try self.transport.write(data: buff)
    }
  }
  
  public func write(_ value: Int64) throws {
    var buff = Data()
    buff.append(Data(bytes: [UInt8(0xff & (value >> 56))]))
    buff.append(Data(bytes: [UInt8(0xff & (value >> 48))]))
    buff.append(Data(bytes: [UInt8(0xff & (value >> 40))]))
    buff.append(Data(bytes: [UInt8(0xff & (value >> 32))]))
    buff.append(Data(bytes: [UInt8(0xff & (value >> 24))]))
    buff.append(Data(bytes: [UInt8(0xff & (value >> 16))]))
    buff.append(Data(bytes: [UInt8(0xff & (value >> 8))]))
    buff.append(Data(bytes: [UInt8(0xff & (value))]))
    
    try ProtocolTransportTry(error: TProtocolError(message: "Transport write failed")) {
      try self.transport.write(data: buff)
    }
  }
  
  public func write(_ value: Double) throws {
    // Notably unsafe, since Double and Int64 are the same size, this should work fine
    try self.write(unsafeBitCast(value, to: Int64.self))
  }
  
  public func write(_ data: Data) throws {
    try write(Int32(data.count))
    
    try ProtocolTransportTry(error: TProtocolError(message: "Transport write failed")) {
      try self.transport.write(data: data)
    }
  }
}
