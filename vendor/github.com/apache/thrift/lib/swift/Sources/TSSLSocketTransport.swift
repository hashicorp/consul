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
public class TSSLSocketTransport {
  init(hostname: String, port: UInt16) {
    // FIXME!
    assert(false, "Security not available in Linux, TSSLSocketTransport Unavilable for now")
  }
}
#else
let isLittleEndian = Int(OSHostByteOrder()) == OSLittleEndian
let htons  = isLittleEndian ? _OSSwapInt16 : { $0 }
let htonl  = isLittleEndian ? _OSSwapInt32 : { $0 }

public class TSSLSocketTransport: TStreamTransport {
  var sslHostname: String
  var sd: Int32 = 0
  
  public init(hostname: String, port: UInt16) throws {
    sslHostname = hostname
    var readStream: Unmanaged<CFReadStream>?
    var writeStream: Unmanaged<CFWriteStream>?
    
    /* create a socket structure */
    var pin: sockaddr_in = sockaddr_in()
    var hp: UnsafeMutablePointer<hostent>? = nil
    for i in 0..<10 {
      
      hp = gethostbyname(hostname.cString(using: String.Encoding.utf8)!)
      if hp == nil {
        print("failed to resolve hostname \(hostname)")
        herror("resolv")
        if i == 9 {
          super.init(inputStream: nil, outputStream: nil) // have to init before throwing
          throw TSSLSocketTransportError(error: .hostanameResolution(hostname: hostname))
        }
        Thread.sleep(forTimeInterval: 0.2)
      } else {
        break
      }
    }
    pin.sin_family  = UInt8(AF_INET)
    pin.sin_addr    = in_addr(s_addr: UInt32((hp?.pointee.h_addr_list.pointee?.pointee)!)) // Is there a better way to get this???
    pin.sin_port    = htons(port)
    
    /* create the socket */
    sd = socket(Int32(AF_INET), Int32(SOCK_STREAM), Int32(IPPROTO_TCP))
    if sd == -1 {
      super.init(inputStream: nil, outputStream: nil) // have to init before throwing
      throw TSSLSocketTransportError(error: .socketCreate(port: Int(port)))
    }
    
    /* open a connection */
    // need a non-self ref to sd, otherwise the j complains
    let sd_local = sd
    let connectResult = withUnsafePointer(to: &pin) {
      connect(sd_local, UnsafePointer<sockaddr>(OpaquePointer($0)), socklen_t(MemoryLayout<sockaddr_in>.size))
    }
    if connectResult == -1 {
      super.init(inputStream: nil, outputStream: nil) // have to init before throwing
      throw TSSLSocketTransportError(error: .connect)
    }
    
    CFStreamCreatePairWithSocket(kCFAllocatorDefault, sd, &readStream, &writeStream)
    
    CFReadStreamSetProperty(readStream?.takeRetainedValue(), .socketNativeHandle, kCFBooleanTrue)
    CFWriteStreamSetProperty(writeStream?.takeRetainedValue(), .socketNativeHandle, kCFBooleanTrue)
    
    var inputStream: InputStream? = nil
    var outputStream: OutputStream? = nil
    if readStream != nil && writeStream != nil {
      
      CFReadStreamSetProperty(readStream?.takeRetainedValue(),
                              .socketSecurityLevel,
                              kCFStreamSocketSecurityLevelTLSv1)
      
      let settings: [String: Bool] = [kCFStreamSSLValidatesCertificateChain as String: true]
      
      CFReadStreamSetProperty(readStream?.takeRetainedValue(),
                              .SSLSettings,
                              settings as CFTypeRef!)
      
      CFWriteStreamSetProperty(writeStream?.takeRetainedValue(),
                              .SSLSettings,
                              settings as CFTypeRef!)
      
      inputStream = readStream!.takeRetainedValue()
      inputStream?.schedule(in: .current, forMode: .defaultRunLoopMode)
      inputStream?.open()
      
      outputStream = writeStream!.takeRetainedValue()
      outputStream?.schedule(in: .current, forMode: .defaultRunLoopMode)
      outputStream?.open()
      
      readStream?.release()
      writeStream?.release()
    }
    
    
    super.init(inputStream: inputStream, outputStream: outputStream)
    self.input?.delegate = self
    self.output?.delegate = self
  }
  
  func recoverFromTrustFailure(_ myTrust: SecTrust, lastTrustResult: SecTrustResultType) -> Bool {
    let trustTime = SecTrustGetVerifyTime(myTrust)
    let currentTime = CFAbsoluteTimeGetCurrent()
    
    let timeIncrement = 31536000 // from TSSLSocketTransport.m
    let newTime = currentTime - Double(timeIncrement)
    
    if trustTime - newTime != 0 {
      let newDate = CFDateCreate(nil, newTime)
      SecTrustSetVerifyDate(myTrust, newDate!)
      
      var tr = lastTrustResult
      let success = withUnsafeMutablePointer(to: &tr) { trPtr -> Bool in
        if SecTrustEvaluate(myTrust, trPtr) != errSecSuccess {
          return false
        }
        return true
      }
      if !success { return false }
    }
    if lastTrustResult == .proceed || lastTrustResult == .unspecified {
        return false
    }

    print("TSSLSocketTransport: Unable to recover certificate trust failure")
    return true
  }
  
  public func isOpen() -> Bool {
    return sd > 0
  }
}

extension TSSLSocketTransport: StreamDelegate {
  public func stream(_ aStream: Stream, handle eventCode: Stream.Event) {
    
    switch eventCode {
    case Stream.Event(): break
    case Stream.Event.hasBytesAvailable: break
    case Stream.Event.openCompleted: break
    case Stream.Event.hasSpaceAvailable:
      var proceed = false
      var trustResult: SecTrustResultType = .invalid

      var newPolicies: CFMutableArray?
      
      repeat {
        let trust: SecTrust = aStream.property(forKey: .SSLPeerTrust) as! SecTrust
        
        // Add new policy to current list of policies
        let policy = SecPolicyCreateSSL(false, sslHostname as CFString?)
        var ppolicy = policy // mutable for pointer
        let policies: UnsafeMutablePointer<CFArray?>? = nil
        if SecTrustCopyPolicies(trust, policies!) != errSecSuccess {
          break
        }
        withUnsafeMutablePointer(to: &ppolicy) { ptr in
          newPolicies = CFArrayCreateMutableCopy(nil, 0, policies?.pointee)
          CFArrayAppendValue(newPolicies, ptr)
        }
        
        // update trust policies
        if SecTrustSetPolicies(trust, newPolicies!) != errSecSuccess {
          break
        }
        
        // Evaluate the trust chain
        let success = withUnsafeMutablePointer(to: &trustResult) { trustPtr -> Bool in
          if SecTrustEvaluate(trust, trustPtr) != errSecSuccess {
            return false
          }
          return true
        }
        
        if !success {
          break
        }
        
        
        switch trustResult {
        case .proceed:      proceed = true
        case .unspecified:  proceed = true
        case .recoverableTrustFailure:
          proceed = self.recoverFromTrustFailure(trust, lastTrustResult: trustResult)
          
        case .deny:         break
        case .fatalTrustFailure: break
        case .otherError:   break
        case .invalid:      break
        default: break
        }
      } while false
  
      if !proceed {
        print("TSSLSocketTransport: Cannot trust certificate.  Result: \(trustResult)")
        aStream.close()
      }
      
    case Stream.Event.errorOccurred: break
    case Stream.Event.endEncountered: break
    default: break
    }
  }
}
#endif
