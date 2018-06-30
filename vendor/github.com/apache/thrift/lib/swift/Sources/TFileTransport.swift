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

#if os(OSX) || os(iOS) || os(watchOS) || os(tvOS)
  import Darwin
#elseif os(Linux) || os(FreeBSD) || os(PS4) || os(Android)
  import Glibc
#endif

/// TFileTransport
/// Foundation-less Swift File transport.
/// Uses C fopen/fread/fwrite,
/// provided by Glibc in linux and Darwin on OSX/iOS
public class TFileTransport: TTransport {
  var fileHandle: UnsafeMutablePointer<FILE>? = nil
  
  public init (fileHandle: UnsafeMutablePointer<FILE>) {
    self.fileHandle = fileHandle
  }

  public convenience init(filename: String) throws {
    var fileHandle: UnsafeMutablePointer<FILE>?
    filename.withCString({ cFilename in
      "rw".withCString({ cMode in
        fileHandle = fopen(cFilename, cMode)
      })
    })
    if let fileHandle = fileHandle {
      self.init(fileHandle: fileHandle)
    } else {
      throw TTransportError(error: .notOpen)
    }
  }
  
  deinit {
    fclose(self.fileHandle)
  }
  
  public func readAll(size: Int) throws -> Data {
    let read = try self.read(size: size)
    
    if read.count != size {
      throw TTransportError(error: .endOfFile)
    }
    return read
  }
  
  public func read(size: Int) throws -> Data {
    // set up read buffer, position 0
    var read = Data(capacity: size)
    var position = 0
    
    // read character buffer
    var nextChar: UInt8 = 0
    
    // continue until we've read size bytes
    while read.count < size {
      if fread(&nextChar, 1, 1, self.fileHandle) == 1 {
        read[position] = nextChar

        // Increment output byte pointer
        position += 1
        
      } else {
        throw TTransportError(error: .endOfFile)
      }
    }
    return read
  }
  
  public func write(data: Data) throws {
    let bytesWritten = data.withUnsafeBytes {
      fwrite($0, 1, data.count, self.fileHandle)
    }
    if bytesWritten != data.count {
      throw TTransportError(error: .unknown)
    }
  }
  
  public func flush() throws {
    return
  }
}
