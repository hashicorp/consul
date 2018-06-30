// Copyright 2018 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import Foundation

// The I/O code below is derived from Apple's swift-protobuf project.
// https://github.com/apple/swift-protobuf
// BEGIN swift-protobuf derivation

#if os(Linux)
  import Glibc
#else
  import Darwin.C
#endif

enum PluginError: Error {
  /// Raised for any errors reading the input
  case readFailure
}

// Alias clib's write() so Stdout.write(bytes:) can call it.
private let _write = write

class Stdin {
  static func readall() throws -> Data {
    let fd: Int32 = 0
    let buffSize = 32
    var buff = [UInt8]()
    while true {
      var fragment = [UInt8](repeating: 0, count: buffSize)
      let count = read(fd, &fragment, buffSize)
      if count < 0 {
        throw PluginError.readFailure
      }
      if count < buffSize {
        buff += fragment[0..<count]
        return Data(bytes: buff)
      }
      buff += fragment
    }
  }
}

class Stdout {
  static func write(bytes: Data) {
    bytes.withUnsafeBytes { (p: UnsafePointer<UInt8>) -> () in
      _ = _write(1, p, bytes.count)
    }
  }
  static func write(_ string: String) {
    self.write(bytes:string.data(using:.utf8)!)
  }
}

class Stderr {
  static func write(bytes: Data) {
    bytes.withUnsafeBytes { (p: UnsafePointer<UInt8>) -> () in
      _ = _write(2, p, bytes.count)
    }
  }
  static func write(_ string: String) {
    self.write(bytes:string.data(using:.utf8)!)
  }
}

// END swift-protobuf derivation
