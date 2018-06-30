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

#if os(Linux)
  /// Currently unavailable in Linux
  /// Remove comments and build to fix
  /// Currently kConstants for CFSockets don't exist in linux and not all have been moved
  /// to property structs yet
#else
  // Must inherit NSObject for NSStreamDelegate conformance
  public class TStreamTransport : NSObject, TTransport {
    public var input: InputStream? = nil
    public var output: OutputStream? = nil
    
    public init(inputStream: InputStream?, outputStream: OutputStream?) {
      input   = inputStream
      output  = outputStream
    }
    
    public convenience init(inputStream: InputStream?) {
      self.init(inputStream: inputStream, outputStream: nil)
    }
    
    public convenience init(outputStream: OutputStream?) {
      self.init(inputStream: nil, outputStream: outputStream)
    }
    
    deinit {
      close()
    }
    
    public func readAll(size: Int) throws -> Data {
      guard let input = input else {
        throw TTransportError(error: .unknown)
      }
      
      var read = Data()
      while read.count < size {
        var buffer = Array<UInt8>(repeating: 0, count: size - read.count)
        
        let bytesRead = buffer.withUnsafeMutableBufferPointer { bufferPtr in
          return input.read(bufferPtr.baseAddress!, maxLength: size - read.count)
        }
        
        if bytesRead <= 0 {
          throw TTransportError(error: .notOpen)
        }
        read.append(Data(bytes: buffer))
      }
      return read
    }
    
    public func read(size: Int) throws -> Data {
      guard let input = input else {
        throw TTransportError(error: .unknown)
      }
      
      var read = Data()
      while read.count < size {
        var buffer = Array<UInt8>(repeating: 0, count: size - read.count)
        let bytesRead = buffer.withUnsafeMutableBufferPointer {
          input.read($0.baseAddress!, maxLength: size - read.count)
        }
        
        if bytesRead <= 0 {
          break
        }
        
        read.append(Data(bytes: buffer))
      }
      return read
    }
    
    public func write(data: Data) throws {
      guard let output = output else {
        throw TTransportError(error: .unknown)
      }
      
      var bytesWritten = 0
      while bytesWritten < data.count {
        bytesWritten = data.withUnsafeBytes {
          return output.write($0, maxLength: data.count)
        }
        
        if bytesWritten == -1 {
          throw TTransportError(error: .notOpen)
        } else if bytesWritten == 0 {
          throw TTransportError(error: .endOfFile)
        }
      }
    }
    
    
    public func flush() throws {
      return
    }
    
    public func close() {
      
      if input != nil {
        // Close and reset inputstream
        if let cf: CFReadStream = input {
          CFReadStreamSetProperty(cf, .shouldCloseNativeSocket, kCFBooleanTrue)
        }
        
        input?.delegate = nil
        input?.close()
        input?.remove(from: .current, forMode: .defaultRunLoopMode)
        input = nil
      }
      
      if output != nil {
        // Close and reset output stream
        if let cf: CFWriteStream = output {
          CFWriteStreamSetProperty(cf, .shouldCloseNativeSocket, kCFBooleanTrue)
        }
        output?.delegate = nil
        output?.close()
        output?.remove(from: .current, forMode: .defaultRunLoopMode)
        output = nil
      }
    }
  }
#endif
