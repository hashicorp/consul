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
import CoreFoundation

public enum TCType: UInt8 {
  case stop          = 0x00
  case boolean_TRUE  = 0x01
  case boolean_FALSE = 0x02
  case i8            = 0x03
  case i16           = 0x04
  case i32           = 0x05
  case i64           = 0x06
  case double        = 0x07
  case binary        = 0x08
  case list          = 0x09
  case set           = 0x0A
  case map           = 0x0B
  case `struct`      = 0x0C
  
  public static let typeMask: UInt8 = 0xE0 // 1110 0000
  public static let typeBits: UInt8 = 0x07 // 0000 0111
  public static let typeShiftAmount = 5
 
}


public class TCompactProtocol: TProtocol {
  public static let protocolID: UInt8  = 0x82
  public static let version: UInt8     = 1
  public static let versionMask: UInt8 = 0x1F // 0001 1111
  
  public var transport: TTransport
  
  var lastField: [UInt8] = []
  var lastFieldId: UInt8 = 0
  
  var boolFieldName: String?
  var boolFieldType: TType?
  var boolFieldId: Int32?
  var booleanValue: Bool?
  
  var currentMessageName: String?

  public required init(on transport: TTransport) {
    self.transport = transport
  }

  
  /// Mark: - TCompactProtocol helpers
  
  func writebyteDirect(_ byte: UInt8) throws {
    let byte = Data(bytes: [byte])
    try ProtocolTransportTry(error: TProtocolError(message: "Transport Write Failed")) {
      try self.transport.write(data: byte)
    }
  }
  
  func writeVarint32(_ val: UInt32) throws {
    var val = val
    var i32buf = [UInt8](repeating: 0, count: 5)
    var idx = 0
    while true {
      if (val & ~0x7F) == 0 {
        i32buf[idx] = UInt8(val)
        idx += 1
        break
      } else {
        i32buf[idx] = UInt8((val & 0x7F) | 0x80)
        idx += 1
        val >>= 7
      }
    }
    
    try ProtocolTransportTry(error: TProtocolError(message: "Transport Write Failed")) {
      try self.transport.write(data: Data(bytes: i32buf[0..<idx]))
    }
  }
  
  func writeVarint64(_ val: UInt64) throws {
    var val = val
    var varint64out = [UInt8](repeating: 0, count: 10)
    var idx = 0
    while true {
      if (val & ~0x7F) == 0{
        varint64out[idx] = UInt8(val)
        idx += 1
        break
      } else {
        varint64out[idx] = UInt8(val & 0x7F) | 0x80
        idx += 1
        val >>= 7
      }
    }
    
    try ProtocolTransportTry(error: TProtocolError(message: "Transport Write Failed")) {
      try self.transport.write(data: Data(bytes: varint64out[0..<idx]))
    }
  
  }
  
  func writeCollectionBegin(_ elementType: TType, size: Int32) throws {
    let ctype = compactType(elementType).rawValue
    if size <= 14 {
      try writebyteDirect(UInt8(size << 4) | ctype)
    } else {
      try writebyteDirect(0xF0 | ctype)
      try writeVarint32(UInt32(size))
    }
  }
  
  func readBinary(_ size: Int) throws -> Data {
    var result = Data()
    if size != 0 {
      try ProtocolTransportTry(error: TProtocolError(message: "Transport Read Failed")) {
        result = try self.transport.readAll(size: size)
      }
    }
    return result
  }
  
  func readVarint32() throws -> UInt32 {
    var result: UInt32 = 0
    var shift: UInt32 = 0
    while true {
      let byte: UInt8 = try read()
      
      result |= UInt32(byte & 0x7F) << shift
      if (byte & 0x80) == 0 {
        break
      }
      
      shift += 7
    }
    
    return result
  }
  
  func readVarint64() throws -> UInt64 {
    var result: UInt64 = 0
    var shift: UInt64 = 0
    
    while true {
      let byte: UInt8 = try read()
      
      result |= UInt64(byte & 0x7F) << shift
      if (byte & 0x80) == 0 {
        break
      }
      
      shift += 7
    }
    return result
  }
  

  func ttype(_ compactTypeVal: UInt8) throws -> TType {
    guard let compactType = TCType(rawValue: compactTypeVal) else {
      throw TProtocolError(message: "Unknown TCType value: \(compactTypeVal)")
    }
    
    switch compactType {
    case .stop: return .stop;
    case .boolean_FALSE, .boolean_TRUE: return .bool;
    case .i8: return .i8;
    case .i16: return .i16;
    case .i32: return .i32;
    case .i64: return .i64;
    case .double: return .double;
    case .binary: return .string;
    case .list: return .list;
    case .set: return .set;
    case .map: return .map;
    case .struct: return .struct;
    }
  }
  
  func compactType(_ ttype: TType) -> TCType {
    switch ttype {
    case .stop:   return .stop
    case .void:   return .i8
    case .bool:   return .boolean_FALSE
    case .i8:   return .i8
    case .double: return .double
    case .i16:    return .i16
    case .i32:    return .i32
    case .i64:    return .i64
    case .string: return .binary
    case .struct: return .struct
    case .map:    return .map
    case .set:    return .set
    case .list:   return .list
    case .utf8:   return .binary
    case .utf16:  return .binary
    }
  }
  
  /// ZigZag encoding maps signed integers to unsigned integers so that
  /// numbers with a small absolute value (for instance, -1) have
  /// a small varint encoded value too. It does this in a way that
  /// "zig-zags" back and forth through the positive and negative integers,
  /// so that -1 is encoded as 1, 1 is encoded as 2, -2 is encoded as 3, and so
  ///
  /// - parameter n: number to zigzag
  ///
  /// - returns: zigzaged UInt32
  func i32ToZigZag(_ n : Int32) -> UInt32 {
    return UInt32(n << 1) ^ UInt32(n >> 31)
  }

  func i64ToZigZag(_ n : Int64) -> UInt64 {
    return UInt64(n << 1) ^ UInt64(n >> 63)
  }

  func zigZagToi32(_ n: UInt32) -> Int32 {
    return Int32(n >> 1) ^ (-Int32(n & 1))
  }
  
  func zigZagToi64(_ n: UInt64) -> Int64 {
    return Int64(n >> 1) ^ (-Int64(n & 1))
  }
  
  
  
  /// Mark: - TProtocol  
  
  public func readMessageBegin() throws -> (String, TMessageType, Int32) {
    let protocolId: UInt8 = try read()
    
    if protocolId != TCompactProtocol.protocolID {
      let expected = String(format:"%2X", TCompactProtocol.protocolID)
      let got      = String(format:"%2X", protocolId)
      throw TProtocolError(message: "Wrong Protocol ID \(got)",
                           extendedError: .mismatchedProtocol(expected: expected, got: got))

    }

    let versionAndType: UInt8 = try read()
    let version: UInt8 = versionAndType & TCompactProtocol.versionMask
    if version != TCompactProtocol.version {
      throw TProtocolError(error: .badVersion(expected: "\(TCompactProtocol.version)",
                                              got:"\(version)"))

    }
    
    let type = (versionAndType >> UInt8(TCType.typeShiftAmount)) & TCType.typeBits
    guard let mtype = TMessageType(rawValue: Int32(type)) else {
      throw TProtocolError(message: "Unknown TMessageType value: \(type)")
    }
    let sequenceId = try readVarint32()
    let name: String = try read()
    
    return (name, mtype, Int32(sequenceId))
  }
  
  public func readMessageEnd() throws { }
  
  public func readStructBegin() throws -> String {
    lastField.append(lastFieldId)
    lastFieldId = 0
    return ""
  }
  
  public func readStructEnd() throws {
    lastFieldId = lastField.last ?? 0
    lastField.removeLast()
  }
  
  public func readFieldBegin() throws -> (String, TType, Int32) {
    let byte: UInt8 = try read()
    guard let type = TCType(rawValue: byte & 0x0F) else {
      throw TProtocolError(message: "Unknown TCType \(byte & 0x0F)")
    }
    
    // if it's a stop, then we can return immediately, as the struct is over
    if type == .stop {
      return ("", .stop, 0)
    }
    
    var fieldId: Int16 = 0
    
    // mask off the 4MSB of the type header.  it could contain a field id delta
    let modifier = (byte & 0xF0) >> 4
    if modifier == 0 {
      // not a delta.  look ahead for the zigzag varint field id
      fieldId = try read()
    } else {
      // has a delta.  add the delta to the last Read field id.
      fieldId = Int16(lastFieldId + modifier)
    }
    
    let fieldType = try ttype(type.rawValue)
    
    // if this happens to be a boolean field, the value is encoded in the type
    if type == .boolean_TRUE || type == .boolean_FALSE {
      // save the boolean value in a special instance variable
      booleanValue = type == .boolean_TRUE
    }
    
    // push the new field onto the field stack so we can keep the deltas going
    lastFieldId = UInt8(fieldId)
    return ("", fieldType, Int32(fieldId))
  }
  
  public func readFieldEnd() throws { }
  
  public func read() throws -> String {
    let length = try readVarint32()
    
    var result: String
    
    if length != 0 {
      let data = try readBinary(Int(length))
      result = String(data: data, encoding: String.Encoding.utf8) ?? ""
    } else {
      result = ""
    }
    
    return result
  }
  
  public func read() throws -> Bool {
    if let val = booleanValue {
      self.booleanValue = nil
      return val
    } else {
      let result = try read() as UInt8
      return TCType(rawValue: result) == .boolean_TRUE
    }
  }
  
  public func read() throws -> UInt8 {
    var buff: UInt8 = 0
    try ProtocolTransportTry(error: TProtocolError(message: "Transport Read Failed")) {
      buff = try self.transport.readAll(size: 1)[0]
    }
    return buff
  }
  
  public func read() throws -> Int16 {
    let v = try readVarint32()
    return Int16(zigZagToi32(v))
  }
  
  public func read() throws -> Int32 {
    let v = try readVarint32()
    return zigZagToi32(v)
  }
  
  public func read() throws -> Int64 {
    let v = try readVarint64()
    return zigZagToi64(v)
  }
  
  public func read() throws -> Double {
    var buff = Data()
    try ProtocolTransportTry(error: TProtocolError(message: "Transport Read Failed")) {
      buff = try self.transport.readAll(size: 8)
    }
    
    let i64: UInt64 = buff.withUnsafeBytes { (ptr: UnsafePointer<UInt8>) -> UInt64 in
      return UnsafePointer<UInt64>(OpaquePointer(ptr)).pointee
    }
    let bits = CFSwapInt64LittleToHost(i64)
    return unsafeBitCast(bits, to: Double.self)
  }
  
  public func read() throws -> Data {
    let length = try readVarint32()
    return try readBinary(Int(length))
  }
  
  public func readMapBegin() throws -> (TType, TType, Int32) {
    var keyAndValueType: UInt8 = 8
    let size = try readVarint32()
    if size != 0 {
      keyAndValueType = try read()
    }
    
    let keyType = try ttype(keyAndValueType >> 4)
    let valueType = try ttype(keyAndValueType & 0xF)
    
    return (keyType, valueType, Int32(size))
  }
  
  public func readMapEnd() throws { }
  
  public func readSetBegin() throws -> (TType, Int32) {
    return try readListBegin()
  }
  
  public func readSetEnd() throws { }
  
  public func readListBegin() throws -> (TType, Int32) {
    let sizeAndType: UInt8 = try read()
    var size: UInt32 = UInt32(sizeAndType >> 4) & 0x0f
    if size == 15 {
      size = try readVarint32()
    }
    let elementType = try ttype(sizeAndType & 0x0F)
    
    return (elementType, Int32(size))
  }
  
  public func readListEnd() throws { }
  
  public func writeMessageBegin(name: String,
                                type messageType: TMessageType,
                                sequenceID: Int32) throws {
    try writebyteDirect(TCompactProtocol.protocolID)
    let nextByte: UInt8 = (TCompactProtocol.version & TCompactProtocol.versionMask) |
                          (UInt8((UInt32(messageType.rawValue) << UInt32(TCType.typeShiftAmount))) &
                          TCType.typeMask)
    try writebyteDirect(nextByte)
    try writeVarint32(UInt32(sequenceID))
    try write(name)
    
    currentMessageName = name
  }
  
  public func writeMessageEnd() throws {
    currentMessageName = nil
  }
  
  public func writeStructBegin(name: String) throws {
    lastField.append(lastFieldId)
    lastFieldId = 0
  }
  
  public func writeStructEnd() throws {
    lastFieldId = lastField.last ?? 0
    lastField.removeLast()
  }
  
  public func writeFieldBegin(name: String,
                              type fieldType: TType,
                              fieldID: Int32) throws {
    if fieldType == .bool {
      boolFieldName = name
      boolFieldType = fieldType
      boolFieldId = fieldID
      return
    } else {
      try writeFieldBeginInternal(name: name,
                                  type: fieldType,
                                  fieldID: fieldID,
                                  typeOverride: 0xFF)
    }
  }
  
  func writeFieldBeginInternal(name: String,
                               type fieldType: TType,
                               fieldID: Int32,
                               typeOverride: UInt8) throws {
    
    let typeToWrite = typeOverride == 0xFF ? compactType(fieldType).rawValue : typeOverride
    
    // check if we can use delta encoding for the field id
    let diff = UInt8(fieldID) - lastFieldId
    if (UInt8(fieldID) > lastFieldId) && (diff <= 15) {
      // Write them together
      try writebyteDirect((UInt8(fieldID) - lastFieldId) << 4 | typeToWrite)
      
    } else {
      // Write them separate
      try writebyteDirect(typeToWrite)
      try write(Int16(fieldID))
    }
    
    lastFieldId = UInt8(fieldID)
      
  }
  
  public func writeFieldStop() throws {
    try writebyteDirect(TCType.stop.rawValue)
  }
  
  public func writeFieldEnd() throws { }
  
  public func writeMapBegin(keyType: TType, valueType: TType, size: Int32) throws {
    if size == 0 {
      try writebyteDirect(0)
    } else {
      try writeVarint32(UInt32(size))
      
      let compactedTypes = compactType(keyType).rawValue << 4 | compactType(valueType).rawValue
      try writebyteDirect(compactedTypes)
    }
  }
  
  public func writeMapEnd() throws { }
  
  public func writeSetBegin(elementType: TType, size: Int32) throws {
    try writeCollectionBegin(elementType, size: size)
  }
  
  public func writeSetEnd() throws { }
  
  public func writeListBegin(elementType: TType, size: Int32) throws {
    try writeCollectionBegin(elementType, size: size)
  }
  
  public func writeListEnd() throws { }
  
  public func write(_ value: String) throws {
    try write(value.data(using: String.Encoding.utf8)!)
  }
  
  public func write(_ value: Bool) throws {
    if let boolFieldId = boolFieldId, let boolFieldType = boolFieldType,
       let boolFieldName = boolFieldName {
      
      // we haven't written the field header yet
      let compactType: TCType = value ? .boolean_TRUE : .boolean_FALSE
      try writeFieldBeginInternal(name: boolFieldName, type: boolFieldType, fieldID: boolFieldId,
                                  typeOverride: compactType.rawValue)
      self.boolFieldId = nil
      self.boolFieldType = nil
      self.boolFieldName = nil
    } else {
      // we're not part of a field, so just write the value.
      try writebyteDirect(value ? TCType.boolean_TRUE.rawValue : TCType.boolean_FALSE.rawValue)
    }
  }

  public func write(_ value: UInt8) throws {
    try writebyteDirect(value)
  }

  public func write(_ value: Int16) throws {
    try writeVarint32(i32ToZigZag(Int32(value)))
  }
  
  public func write(_ value: Int32) throws {
    try writeVarint32(i32ToZigZag(value))
  }
  
  public func write(_ value: Int64) throws {
    try writeVarint64(i64ToZigZag(value))
  }
  
  public func write(_ value: Double) throws {
    var bits = CFSwapInt64HostToLittle(unsafeBitCast(value, to: UInt64.self))
    let data = withUnsafePointer(to: &bits) {
      return Data(bytes: UnsafePointer<UInt8>(OpaquePointer($0)), count: MemoryLayout<UInt64>.size)
    }
    try ProtocolTransportTry(error: TProtocolError(message: "Transport Write Failed")) {
      try self.transport.write(data: data)
    }
  }
  
  public func write(_ data: Data) throws {
    try writeVarint32(UInt32(data.count))
    try ProtocolTransportTry(error: TProtocolError(message: "Transport Write Failed")) {
      try self.transport.write(data: data)
    }
  }
}
