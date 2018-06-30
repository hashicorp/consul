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

public struct TList<Element : TSerializable> : RandomAccessCollection, MutableCollection, ExpressibleByArrayLiteral, TSerializable, Hashable {
  typealias Storage = Array<Element>
  public typealias Indices = Storage.Indices

  internal var storage = Storage()
  public init() { }
  public init(arrayLiteral elements: Element...) {
    self.storage = Storage(elements)
  }
  public init<Source : Sequence>(_ sequence: Source) where Source.Iterator.Element == Element {
    storage = Storage(sequence)
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
  public static var thriftType : TType { return .list }

  public static func read(from proto: TProtocol) throws -> TList {
    let (elementType, size) = try proto.readListBegin()
    if elementType != Element.thriftType {
      throw TProtocolError(error: .invalidData,
                           extendedError: .unexpectedType(type: elementType))
    }
    var list = TList()
    for _ in 0..<size {
      let element = try Element.read(from: proto)
      list.storage.append(element)
    }
    try proto.readListEnd()
    return list
  }
  
  public func write(to proto: TProtocol) throws {
    try proto.writeListBegin(elementType: Element.thriftType, size: Int32(self.count))
    for element in self.storage {
      try Element.write(element, to: proto)
    }
    try proto.writeListEnd()
  }

  /// Mark: MutableCollection
  
  public typealias SubSequence = Storage.SubSequence
  public typealias Index = Storage.Index
  
  public subscript(position: Storage.Index) -> Element {
    get {
      return storage[position]
    }
    set {
      storage[position] = newValue
    }
  }
  
  public subscript(range: Range<Index>) -> SubSequence {
    get {
      return storage[range]
    }
    set {
      storage[range] = newValue
    }
  }
  
  public var startIndex: Index {
    return storage.startIndex
  }
  public var endIndex: Index {
    return storage.endIndex
  }
  
  public func formIndex(after i: inout Index) {
    storage.formIndex(after: &i)
  }
  
  public func formIndex(before i: inout Int) {
    storage.formIndex(before: &i)
  }
  
  public func index(after i: Index) -> Index {
    return storage.index(after: i)
  }

  public func index(before i: Int) -> Int {
    return storage.index(before: i)
  }

}

extension TList : RangeReplaceableCollection {
  public mutating func replaceSubrange<C: Collection>(_ subrange: Range<Index>, with newElements: C)
    where C.Iterator.Element == Element {
    storage.replaceSubrange(subrange, with: newElements)
  }
}

extension TList : CustomStringConvertible, CustomDebugStringConvertible {
  
  public var description : String {
    return storage.description
  }
  
  public var debugDescription : String {
    return storage.debugDescription
  }
  
}

public func ==<Element>(lhs: TList<Element>, rhs: TList<Element>) -> Bool {
  return lhs.storage.elementsEqual(rhs.storage) { $0 == $1 }
}
