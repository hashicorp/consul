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


public protocol TSerializable {
  var hashValue: Int { get }

  /// TType for instance
  static var thriftType: TType { get }

  /// Read TSerializable instance from Protocol
  static func read(from proto: TProtocol) throws -> Self

  /// Write TSerializable instance to Protocol
  func write(to proto: TProtocol) throws

}

extension TSerializable {
  public static func write(_ value: Self, to proto: TProtocol) throws {
    try value.write(to: proto)
  }

  /// convenience for member access
  public var thriftType: TType { return Self.thriftType }
}

public func ==<T>(lhs: T, rhs: T) -> Bool where T : TSerializable {
  return lhs.hashValue == rhs.hashValue
}

/// Default read/write for primitave Thrift types:
/// Bool, Int8 (byte), Int16, Int32, Int64, Double, String

extension Bool : TSerializable {
  public static var thriftType: TType { return .bool }

  public static func read(from proto: TProtocol) throws -> Bool {
    return try proto.read()
  }

  public func write(to proto: TProtocol) throws {
    try proto.write(self)
  }
}

extension Int8 : TSerializable {
  public static var thriftType: TType { return .i8 }

  public static func read(from proto: TProtocol) throws -> Int8 {
    return Int8(try proto.read() as UInt8)
  }

  public func write(to proto: TProtocol) throws {
    try proto.write(UInt8(self))
  }
}

extension Int16 : TSerializable {
  public static var thriftType: TType { return .i16 }

  public static func read(from proto: TProtocol) throws -> Int16 {
    return try proto.read()
  }

  public func write(to proto: TProtocol) throws {
    try proto.write(self)
  }
}

extension Int32 : TSerializable {
  public static var thriftType: TType { return .i32 }

  public static func read(from proto: TProtocol) throws -> Int32 {
    return try proto.read()
  }

  public func write(to proto: TProtocol) throws {
    try proto.write(self)
  }
}


extension Int64 : TSerializable {
  public static var thriftType: TType { return .i64 }

  public static func read(from proto: TProtocol) throws -> Int64 {
    return try proto.read()
  }

  public func write(to proto: TProtocol) throws {
    try proto.write(self)
  }
}

extension Double : TSerializable {
  public static var thriftType: TType { return .double }

  public static func read(from proto: TProtocol) throws -> Double {
    return try proto.read()
  }

  public func write(to proto: TProtocol) throws {
    try proto.write(self)
  }
}

extension String : TSerializable {
  public static var thriftType: TType { return .string }

  public static func read(from proto: TProtocol) throws -> String {
    return try proto.read()
  }

  public func write(to proto: TProtocol) throws {
    try proto.write(self)
  }
}
