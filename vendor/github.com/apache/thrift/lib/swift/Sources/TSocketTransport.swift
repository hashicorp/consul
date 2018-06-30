
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


#if os(OSX) || os(iOS) || os(watchOS) || os(tvOS)
  import Darwin
#elseif os(Linux) || os(FreeBSD) || os(PS4) || os(Android)
  import Glibc
  import Dispatch
#endif

import Foundation
import CoreFoundation

private struct Sys {
  #if os(Linux)
  static let read = Glibc.read
  static let write = Glibc.write
  static let close = Glibc.close
  #else
  static let read = Darwin.read
  static let write = Darwin.write
  static let close = Darwin.close
  #endif
}

extension in_addr {
  public init?(hostent: hostent?) {
    guard let host = hostent, host.h_addr_list != nil, host.h_addr_list.pointee != nil else {
      return nil
    }
    self.init()
    memcpy(&self, host.h_addr_list.pointee!, Int(host.h_length))
    
  }
}


#if os(Linux)
  /// TCFSocketTransport currently unavailable
  /// remove comments and build to see why/fix
  /// currently CF[Read|Write]Stream's can't cast to [Input|Output]Streams which breaks thigns
#else
extension Stream.PropertyKey {
  static let SSLPeerTrust = Stream.PropertyKey(kCFStreamPropertySSLPeerTrust as String)
}

/// TCFSocketTransport, uses CFSockets and (NS)Stream's
public class TCFSocketTransport: TStreamTransport {
    public init?(hostname: String, port: Int, secure: Bool = false) {
    
    var inputStream: InputStream
    var outputStream: OutputStream
    
    var readStream:  Unmanaged<CFReadStream>?
    var writeStream:  Unmanaged<CFWriteStream>?
    CFStreamCreatePairWithSocketToHost(kCFAllocatorDefault,
                                       hostname as CFString!,
                                       UInt32(port),
                                       &readStream,
                                       &writeStream)
    
    if let readStream = readStream?.takeRetainedValue(),
      let writeStream = writeStream?.takeRetainedValue() {
        CFReadStreamSetProperty(readStream, .shouldCloseNativeSocket, kCFBooleanTrue)
        CFWriteStreamSetProperty(writeStream, .shouldCloseNativeSocket, kCFBooleanTrue)
      
        if secure {
            CFReadStreamSetProperty(readStream, .socketSecurityLevel, StreamSocketSecurityLevel.negotiatedSSL._rawValue)
            CFWriteStreamSetProperty(writeStream, .socketSecurityLevel, StreamSocketSecurityLevel.negotiatedSSL._rawValue)
        }

      inputStream = readStream as InputStream
      inputStream.schedule(in: .current, forMode: .defaultRunLoopMode)
      inputStream.open()
      
      outputStream = writeStream as OutputStream
      outputStream.schedule(in: .current, forMode: .defaultRunLoopMode)
      outputStream.open()
      
    } else {
      
      if readStream != nil {
        readStream?.release()
      }
      if writeStream != nil {
        writeStream?.release()
      }
      super.init(inputStream: nil, outputStream: nil)
      return nil
    }
    
    super.init(inputStream: inputStream, outputStream: outputStream)
    
    self.input?.delegate = self
    self.output?.delegate = self
  }
}

extension TCFSocketTransport: StreamDelegate { }
#endif


/// TSocketTransport, posix sockets.  Supports IPv4 only for now
public class TSocketTransport : TTransport {
  public var socketDescriptor: Int32
  
  
  
  /// Initialize from an already set up socketDescriptor.
  /// Expects socket thats already bound/connected (i.e. from listening)
  ///
  /// - parameter socketDescriptor: posix socket descriptor (Int32)
  public init(socketDescriptor: Int32) {
    self.socketDescriptor = socketDescriptor
  }
  
  
  public convenience init(hostname: String, port: Int) throws {
    guard let hp = gethostbyname(hostname.cString(using: .utf8)!)?.pointee,
      let hostAddr = in_addr(hostent: hp) else {
        throw TTransportError(error: .unknown, message: "Invalid address: \(hostname)")
    }
    
    
    
    #if os(Linux)
      let sock = socket(AF_INET, Int32(SOCK_STREAM.rawValue), 0)
      var addr = sockaddr_in(sin_family: sa_family_t(AF_INET),
                             sin_port: in_port_t(htons(UInt16(port))),
                             sin_addr: hostAddr,
                             sin_zero: (0, 0, 0, 0, 0, 0, 0, 0))
    #else
      let sock = socket(AF_INET, SOCK_STREAM, 0)
      
      var addr = sockaddr_in(sin_len: UInt8(MemoryLayout<sockaddr_in>.size),
                             sin_family: sa_family_t(AF_INET),
                             sin_port: in_port_t(htons(UInt16(port))),
                             sin_addr: in_addr(s_addr: in_addr_t(0)),
                             sin_zero: (0, 0, 0, 0, 0, 0, 0, 0))
      
    #endif
    
    let addrPtr = withUnsafePointer(to: &addr){ UnsafePointer<sockaddr>(OpaquePointer($0)) }
    
    let connected = connect(sock, addrPtr, UInt32(MemoryLayout<sockaddr_in>.size))
    if connected != 0 {
      throw TTransportError(error: .notOpen, message: "Error binding to host: \(hostname) \(port)")
    }
    
    self.init(socketDescriptor: sock)
  }
  
  deinit {
    close()
  }
  
  public func readAll(size: Int) throws -> Data {
    var out = Data()
    while out.count < size {
      out.append(try self.read(size: size))
    }
    return out
  }
  
  public func read(size: Int) throws -> Data {
    var buff = Array<UInt8>.init(repeating: 0, count: size)
    let readBytes = Sys.read(socketDescriptor, &buff, size)
    
    return Data(bytes: buff[0..<readBytes])
  }
  
  public func write(data: Data) {
    var bytesToWrite = data.count
    var writeBuffer = data
    while bytesToWrite > 0 {
      let written = writeBuffer.withUnsafeBytes {
        Sys.write(socketDescriptor, $0, writeBuffer.count)
      }
      writeBuffer = writeBuffer.subdata(in: written ..< writeBuffer.count)
      bytesToWrite -= written
    }
  }
  
  public func flush() throws {
    // nothing to do
  }
  
  public func close() {
    shutdown(socketDescriptor, Int32(SHUT_RDWR))
    _ = Sys.close(socketDescriptor)
  }
}
