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

public class TFileHandleTransport: TTransport {
  var inputFileHandle: FileHandle
  var outputFileHandle: FileHandle
  
  public init(inputFileHandle: FileHandle, outputFileHandle: FileHandle) {
    self.inputFileHandle = inputFileHandle
    self.outputFileHandle = outputFileHandle
  }
  
  public convenience init(fileHandle: FileHandle) {
    self.init(inputFileHandle: fileHandle, outputFileHandle: fileHandle)
  }
  
  public func read(size: Int) throws -> Data {
    var data = Data()
    while data.count < size {
      let read = inputFileHandle.readData(ofLength: size - data.count)
      data.append(read)
      if read.count == 0 {
        break
      }
    }
    return data
  }
  
  public func write(data: Data) throws {
    outputFileHandle.write(data)
  }
  
  public func flush() throws {
    return
  }
}


