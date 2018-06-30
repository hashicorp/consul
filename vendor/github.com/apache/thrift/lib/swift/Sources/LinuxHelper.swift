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
/// Extensions for Linux for incomplete Foundation API's.
/// swift-corelibs-foundation is not yet 1:1 with OSX/iOS Foundation

extension CFSocketError {
  public static let success = kCFSocketSuccess
}
  
extension UInt {
  public static func &(lhs: UInt, rhs: Int) -> UInt {
    let cast = unsafeBitCast(rhs, to: UInt.self)
    return lhs & cast
  }
}

#else
extension CFStreamPropertyKey {
  static let shouldCloseNativeSocket  = CFStreamPropertyKey(kCFStreamPropertyShouldCloseNativeSocket)
  // Exists as Stream.PropertyKey.socketSecuritylevelKey but doesn't work with CFReadStreamSetProperty
  static let socketSecurityLevel      = CFStreamPropertyKey(kCFStreamPropertySocketSecurityLevel)
  static let SSLSettings              = CFStreamPropertyKey(kCFStreamPropertySSLSettings)
}
#endif
