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
//

public enum TMessageType: Int32 {
  case call = 1
  case reply = 2
  case exception = 3
  case oneway = 4
}

public enum TType: Int32 {
  case stop     = 0
  case void     = 1
  case bool     = 2
  case i8       = 3
  case double   = 4
  case i16      = 6
  case i32      = 8
  case i64      = 10
  case string   = 11
  case `struct` = 12
  case map      = 13
  case set      = 14
  case list     = 15
  case utf8     = 16
  case utf16    = 17
}

public protocol TProtocol {
  var transport: TTransport { get set }
  init(on transport: TTransport)
  // Reading Methods
  
  func readMessageBegin() throws -> (String, TMessageType, Int32)
  func readMessageEnd() throws
  func readStructBegin() throws -> String
  func readStructEnd() throws
  func readFieldBegin() throws -> (String, TType, Int32)
  func readFieldEnd() throws
  func readMapBegin() throws -> (TType, TType, Int32)
  func readMapEnd() throws
  func readSetBegin() throws -> (TType, Int32)
  func readSetEnd() throws
  func readListBegin() throws -> (TType, Int32)
  func readListEnd() throws
  
  func read() throws -> String
  func read() throws -> Bool
  func read() throws -> UInt8
  func read() throws -> Int16
  func read() throws -> Int32
  func read() throws -> Int64
  func read() throws -> Double
  func read() throws -> Data
  
  // Writing methods
  
  func writeMessageBegin(name: String, type messageType: TMessageType, sequenceID: Int32) throws
  func writeMessageEnd() throws
  func writeStructBegin(name: String) throws
  func writeStructEnd() throws
  func writeFieldBegin(name: String, type fieldType: TType, fieldID: Int32) throws
  func writeFieldStop() throws
  func writeFieldEnd() throws
  func writeMapBegin(keyType: TType, valueType: TType, size: Int32) throws
  func writeMapEnd() throws
  func writeSetBegin(elementType: TType, size: Int32) throws
  func writeSetEnd() throws
  func writeListBegin(elementType: TType, size: Int32) throws
  func writeListEnd() throws

  func write(_ value: String) throws
  func write(_ value: Bool) throws
  func write(_ value: UInt8) throws
  func write(_ value: Int16) throws
  func write(_ value: Int32) throws
  func write(_ value: Int64) throws
  func write(_ value: Double) throws
  func write(_ value: Data) throws
}

public extension TProtocol {
  public func writeFieldValue(_ value: TSerializable, name: String, type: TType, id: Int32) throws {
    try writeFieldBegin(name: name, type: type, fieldID: id)
    try value.write(to: self)
    try writeFieldEnd()
  }

  public func validateValue(_ value: Any?, named name: String) throws {
    if value == nil {
      throw TProtocolError(error: .unknown, message: "Missing required value for field: \(name)")
    }
  }
  
  public func readResultMessageBegin() throws {
    let (_, type, _) = try readMessageBegin();
    if type == .exception {
      let x = try readException()
      throw x
    }
    return
  }
  
  public func readException() throws -> TApplicationError {
    return try TApplicationError.read(from: self)
  }
  
  public func writeException(messageName name: String, sequenceID: Int32, ex: TApplicationError) throws {
    try writeMessageBegin(name: name, type: .exception, sequenceID: sequenceID)
    try ex.write(to: self)
    try writeMessageEnd()
  }
  
  public func skip(type: TType) throws {
    switch type {
    case .bool:   _ = try read() as Bool
    case .i8:   _ = try read() as UInt8
    case .i16:    _ = try read() as Int16
    case .i32:    _ = try read() as Int32
    case .i64:    _ = try read() as Int64
    case .double: _ = try read() as Double
    case .string: _ = try read() as String
      
    case .struct:
      _ = try readStructBegin()
      while true {
        let (_, fieldType, _) = try readFieldBegin()
        if fieldType == .stop {
          break
        }
        try skip(type: fieldType)
        try readFieldEnd()
      }
      try readStructEnd()
      
      
    case .map:
      let (keyType, valueType, size) = try readMapBegin()
      for _ in 0..<size {
        try skip(type: keyType)
        try skip(type: valueType)
      }
      try readMapEnd()
      
      
    case .set:
      let (elemType, size) = try readSetBegin()
      for _ in 0..<size {
        try skip(type: elemType)
      }
      try readSetEnd()
      
    case .list:
      let (elemType, size) = try readListBegin()
      for _ in 0..<size {
        try skip(type: elemType)
      }
      try readListEnd()
    default:
      return
    }
  }
}
