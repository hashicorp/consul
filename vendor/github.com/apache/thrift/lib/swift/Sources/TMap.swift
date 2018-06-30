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

public struct TMap<Key : TSerializable & Hashable, Value : TSerializable>: Collection, ExpressibleByDictionaryLiteral, Hashable, TSerializable {
  typealias Storage = Dictionary<Key, Value>
  public typealias Element = Storage.Element
  public typealias Index = Storage.Index
  public typealias IndexDistance = Storage.IndexDistance
  public typealias Indices = Storage.Indices
  public typealias SubSequence = Storage.SubSequence
  internal var storage = Storage()
  
  /// Mark: Be Like Dictionary
  
  public func indexForKey(_ key: Key) -> Index? {
    return storage.index(forKey: key)
  }
  
  public mutating func updateValue(_ value: Value, forKey key: Key) -> Value? {
    return updateValue(value, forKey: key)
  }
  
  public mutating func removeAtIndex(_ index: DictionaryIndex<Key, Value>) -> (Key, Value) {
    return removeAtIndex(index)
  }
  
  public mutating func removeValueForKey(_ key: Key) -> Value? {
    return storage.removeValue(forKey: key)
  }
  
  public init(minimumCapacity: Int) {
    storage = Storage(minimumCapacity: minimumCapacity)
  }
  
  /// init from Dictionary<K,V>
  public init(_ dict: [Key: Value]) {
    storage = dict
  }

  /// read only access to storage if needed as Dictionary<K,V>
  public var dictionary: [Key: Value] {
    return storage
  }
  
  public subscript (key: Key) -> Value? {
    get {
      return storage[key]
    }
    set {
      storage[key] = newValue
    }
  }
  
  /// Mark: Collection
  
  public var indices: Indices {
    return storage.indices
  }
  
  public func distance(from start: Index, to end: Index) -> IndexDistance {
    return storage.distance(from: start, to: end)
  }
  
  public func index(_ i: Index, offsetBy n: IndexDistance) -> Index {
    return storage.index(i, offsetBy: n)
  }
  
  public func index(_ i: Index, offsetBy n: IndexDistance, limitedBy limit: Index) -> Index? {
    return storage.index(i, offsetBy: n, limitedBy: limit)
  }
  
  public subscript(position: Index) -> Element {
    return storage[position]
  }
  
  /// Mark: IndexableBase
  
  public var startIndex: Index { return storage.startIndex }
  public var endIndex: Index { return storage.endIndex }
  public func index(after i: Index) -> Index {
    return storage.index(after: i)
  }
  
  public func formIndex(after i: inout Index) {
    storage.formIndex(after: &i)
  }
  
  public subscript(bounds: Range<Index>) -> SubSequence {
    return storage[bounds]
  }
  
  /// Mark: DictionaryLiteralConvertible
  
  public init(dictionaryLiteral elements: (Key, Value)...) {
    storage = Storage()
    for (key, value) in elements {
      storage[key] = value
    }
  }

  /// Mark: Hashable
  
  public var hashValue: Int {
    let prime = 31
    var result = 1
    for (key, value) in storage {
      result = prime &* result &+ key.hashValue
      result = prime &* result &+ value.hashValue
    }
    return result
  }
  
  /// Mark: TSerializable
  
  public static var thriftType : TType { return .map }
  public init() {
    storage = Storage()
  }
   
  public static func read(from proto: TProtocol) throws -> TMap {

    let (keyType, valueType, size) = try proto.readMapBegin()
    if size > 0 {
      if keyType != Key.thriftType {
        throw TProtocolError(error: .invalidData,
                             message: "Unexpected TMap Key Type",
                             extendedError: .unexpectedType(type: keyType))
      }
      if valueType != Value.thriftType {
        throw TProtocolError(error: .invalidData,
                             message: "Unexpected TMap Value Type",
                             extendedError: .unexpectedType(type: valueType))
      }
    }

    var map = TMap()
    for _ in 0..<size {
      let key = try Key.read(from: proto)
      let value = try Value.read(from: proto)
      map.storage[key] = value
    }
    try proto.readMapEnd()
    return map
  }
  
  public func write(to proto: TProtocol) throws {
    try proto.writeMapBegin(keyType: Key.thriftType,
                            valueType: Value.thriftType, size: Int32(self.count))
    for (key, value) in self.storage {
      try Key.write(key, to: proto)
      try Value.write(value, to: proto)
    }
    try proto.writeMapEnd()
  }
}

/// Mark: CustomStringConvertible, CustomDebugStringConvertible

extension TMap : CustomStringConvertible, CustomDebugStringConvertible {
  
  public var description : String {
    return storage.description
  }
  
  public var debugDescription : String {
    return storage.debugDescription
  }
  
}

/// Mark: Equatable

public func ==<Key, Value>(lhs: TMap<Key,Value>, rhs: TMap<Key, Value>) -> Bool {
  if lhs.count != rhs.count {
    return false
  }
  return lhs.storage.elementsEqual(rhs.storage) { $0.key == $1.key && $0.value == $1.value }
}
