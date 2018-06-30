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
import Dispatch


public class THTTPSessionTransport: TAsyncTransport {
  public class Factory : TAsyncTransportFactory {
    public var responseValidate: ((HTTPURLResponse?, Data?) throws -> Void)?
    
    var session: URLSession
    var url: URL
    
    public class func setupDefaultsForSessionConfiguration(_ config: URLSessionConfiguration, withProtocolName protocolName: String?) {
      var thriftContentType = "application/x-thrift"
      
      if let protocolName = protocolName {
        thriftContentType += "; p=\(protocolName)"
      }
      
      config.requestCachePolicy = .reloadIgnoringLocalCacheData
      config.urlCache = nil
      
      config.httpShouldUsePipelining  = true
      config.httpShouldSetCookies     = true
      config.httpAdditionalHeaders    = ["Content-Type": thriftContentType,
                                         "Accept": thriftContentType,
                                         "User-Agent": "Thrift/Swift (Session)"]
      
      
    }
    
    public init(session: URLSession, url: URL) {
      self.session = session
      self.url = url
    }
    
    public func newTransport() -> THTTPSessionTransport {
      return THTTPSessionTransport(factory: self)
    }
    
    func validateResponse(_ response: HTTPURLResponse?, data: Data?) throws {
      try responseValidate?(response, data)
    }
    
    func taskWithRequest(_ request: URLRequest, completionHandler: @escaping (Data?, URLResponse?, Error?) -> ()) throws -> URLSessionTask {
      
      let newTask: URLSessionTask? = session.dataTask(with: request, completionHandler: completionHandler)
      if let newTask = newTask {
        return newTask
      } else {
        throw TTransportError(error: .unknown, message: "Failed to create session data task")
      }
    }    
  }
  
  var factory: Factory
  var requestData = Data()
  var responseData = Data()
  var responseDataOffset: Int = 0
  
  init(factory: Factory) {
    self.factory = factory
  }
  
  public func readAll(size: Int) throws -> Data {
    let read = try self.read(size: size)
    if read.count != size {
      throw TTransportError(error: .endOfFile)
    }
    return read
  }
  
  public func read(size: Int) throws -> Data {
    let avail = responseData.count - responseDataOffset
    let (start, stop) = (responseDataOffset, responseDataOffset + min(size, avail))
    let read = responseData.subdata(in: start..<stop)
    responseDataOffset += read.count
    return read
  }
  
  public func write(data: Data) throws {
    requestData.append(data)
  }
  
  public func flush(_ completed: @escaping (TAsyncTransport, Error?) -> Void) {
    var error: Error?
    var task: URLSessionTask?
    
    var request = URLRequest(url: factory.url)
    request.httpMethod = "POST"
    request.httpBody =  requestData

    requestData = Data()

    do {
      task = try factory.taskWithRequest(request, completionHandler: { (data, response, taskError) in

        // Check if there was an error with the network
        if taskError != nil {
            error = TTransportError(error: .timedOut)
            completed(self, error)
            return
        }

        // Check response type
        if taskError == nil && !(response is HTTPURLResponse) {
            error = THTTPTransportError(error: .invalidResponse)
            completed(self, error)
            return
        }
        
        // Check status code
        if let httpResponse = response as? HTTPURLResponse {
          if taskError == nil && httpResponse.statusCode != 200 {
            if httpResponse.statusCode == 401 {
              error = THTTPTransportError(error: .authentication)
            } else {
              error = THTTPTransportError(error: .invalidStatus(statusCode: httpResponse.statusCode))
            }
          }
          
          // Allow factory to check
          if error != nil {
            do {
              try self.factory.validateResponse(httpResponse, data: data)
            } catch let validateError {
              error = validateError
            }
          }
          
          self.responseDataOffset = 0
          if error != nil {
            self.responseData = Data()
          } else {
            self.responseData = data ?? Data()
          }
          completed(self, error)
        }
      })
      
    } catch let taskError {
      error = taskError
    }
    
    if let error = error, task == nil {
      completed(self, error)
    }
    task?.resume()
  }

  public func flush() throws {
    let completed = DispatchSemaphore(value: 0)
    var internalError: Error?
    
    flush() { _, error in
      internalError = error
      completed.signal()
    }
    
    _ = completed.wait(timeout: DispatchTime.distantFuture)
    
    if let error = internalError {
      throw error
    }
  }
}
