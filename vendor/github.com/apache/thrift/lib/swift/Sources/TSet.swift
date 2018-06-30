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

public struct TSet<Element : TSerializable & Hashable> : SetAlgebra, Hashable, Collection, ExpressibleByArrayLiteral, TSerializable {
  /// Typealias for Storage type
  typealias Storage = Set<Element>
  
  
  /// Internal Storage used for TSet (Set\<Element\>)
  internal var storage : Storage
  
  
  /// Mark: Collection
  
  public typealias Indices = Storage.Indices
  public typealias Index = Storage.Index
  public typealias IndexDistance = Storage.IndexDistance
  public typealias SubSequence = Storage.SubSequence
  
  
  public var indices: Indices { return storage.indices }
  
  // Must implement isEmpty even though both SetAlgebra and Collection provide it due to their conflciting default implementations
  public var isEmpty: Bool { return storage.isEmpty }
  
  public func distance(from start: Index, to end: Index) -> IndexDistance {
    return storage.distance(from: start, to: end)
  }
  
  public func index(_ i: Index, offsetBy n: IndexDistance) -> Index {
    return storage.index(i, offsetBy: n)
  }
  
  public func index(_ i: Index, offsetBy n: IndexDistance, limitedBy limit: Index) -> Index? {
    return storage.index(i, offsetBy: n, limitedBy: limit)
  }
  
  #if swift(>=3.2)
  public subscript (position: Storage.Index) -> Element {
      return storage[position]
    }
  #else
  public subscript (position: Storage.Index) -> Element? {
    return storage[position]
  }
  #endif
  
  /// Mark: SetAlgebra
  internal init(storage: Set<Element>) {
    self.storage = storage
  }
  
  public func contains(_ member: Element) -> Bool {
    return storage.contains(member)
  }
  
  public mutating func insert(_ newMember: Element) -> (inserted: Bool, memberAfterInsert: Element) {
    return storage.insert(newMember)
  }
  
  public mutating func remove(_ member: Element) -> Element? {
    return storage.remove(member)
  }
  
  public func union(_ other: TSet<Element>) -> TSet {
    return TSet(storage: storage.union(other.storage))
  }
  
  public mutating func formIntersection(_ other: TSet<Element>) {
    return storage.formIntersection(other.storage)
  }
  
  public mutating func formSymmetricDifference(_ other: TSet<Element>) {
    return storage.formSymmetricDifference(other.storage)
  }
  
  public mutating func formUnion(_ other: TSet<Element>) {
    return storage.formUnion(other.storage)
  }
  
  public func intersection(_ other: TSet<Element>) -> TSet {
    return TSet(storage: storage.intersection(other.storage))
  }
  
  public func symmetricDifference(_ other: TSet<Element>) -> TSet {
    return TSet(storage: storage.symmetricDifference(other.storage))
  }
  
  public mutating func update(with newMember: Element) -> Element? {
    return storage.update(with: newMember)
  }
  
  /// Mark: IndexableBase
  
  public var startIndex: Index { return storage.startIndex }
  public var endIndex: Index { return storage.endIndex }
  public func index(after i: Index) -> Index {
    return storage.index(after: i)
  }

  public func formIndex(after i: inout Storage.Index) {
    storage.formIndex(after: &i)
  }
  
  public subscript(bounds: Range<Index>) -> SubSequence {
    return storage[bounds]
  }

  
  /// Mark: Hashable
  public var hashValue : Int {
    let prime = 31
    var result = 1
    for element in storage {
      result = prime &* result &+ element.hashValue
    }
    return result
  }
  
  /// Mark: TSerializable
  public static var thriftType : TType { return .set }
  
  public init() {
    storage = Storage()
  }
  
  public init(arrayLiteral elements: Element...) {
    self.storage = Storage(elements)
  }
  
  public init<Source : Sequence>(_ sequence: Source) where Source.Iterator.Element == Element {
    storage = Storage(sequence)
  }
  
  public static func read(from proto: TProtocol) throws -> TSet {
    let (elementType, size) = try proto.readSetBegin()
    if elementType != Element.thriftType {
      throw TProtocolError(error: .invalidData,
                           extendedError: .unexpectedType(type: elementType))
    }
    var set = TSet()
    for _ in 0..<size {
      let element = try Element.read(from: proto)
      set.storage.insert(element)
    }
    try proto.readSetEnd()
    return set
  }
  
  public func write(to proto: TProtocol) throws {
    try proto.writeSetBegin(elementType: Element.thriftType, size: Int32(self.count))
    for element in self.storage {
      try Element.write(element, to: proto)
    }
    try proto.writeSetEnd()
  }
}

extension TSet: CustomStringConvertible, CustomDebugStringConvertible {
  public var description : String {
    return storage.description
  }
  public var debugDescription : String {
    return storage.debugDescription
  }
  
}

public func ==<Element>(lhs: TSet<Element>, rhs: TSet<Element>) -> Bool {
  return lhs.storage == rhs.storage
}
