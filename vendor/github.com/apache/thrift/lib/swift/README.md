Thrift Swift Library
=========================

License
-------
Licensed to the Apache Software Foundation (ASF) under one
or more contributor license agreements. See the NOTICE file
distributed with this work for additional information
regarding copyright ownership. The ASF licenses this file
to you under the Apache License, Version 2.0 (the
"License"); you may not use this file except in compliance
with the License. You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing,
software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
KIND, either express or implied. See the License for the
specific language governing permissions and limitations
under the License.


## Build

    swift build

## Test
    swift test

## Install Library
##### Cocoapods
Add the following to your podfile
```ruby
    pod 'Thrift-swift3', :git => 'git@github.com:apache/thrift.git', :branch => 'master'
```

##### SPM
Unfortunately due to some limitations in SPM, the Package manifest and Sources directory must be at the root of the project.
To get around that for the time being, you can use this mirrored repo.
Add the following to your Package.swift
```swift
dependencies: [
    .Package(url: "https://github.com/apocolipse/Thrift-Swift.git", majorVersion: 1)
]
```

## Thrift Compiler

You can compile IDL sources for Swift 3 with the following command:

    thrift --gen swift thrift_file

## Client Example
```swift
let transport = TSocketTransport(hostname: "localhost", port: 9090)!

//  var proto = TCompactProtocol(transport: transport)
let proto = TBinaryProtocol(on: transport)
//  var client = HermesClient(inoutProtocol: proto)
let client = ThriftTestClient(inoutProtocol: proto)
do {
    try client.testVoid()
} catch let error {
    print("\(error)")
}
```

## Library Notes
- Eliminated Protocol Factories, They were only used in async clients and server implementations, where Generics provide a better alternative.
- Swifty Errors, All `TError` types have a nested `ErrorCode` Enum as well as some extra flavor where needed.
- Value typed everything.  `TTransport` operates on value typed `Data` rather than reference typed `NSData` or `UnsafeBufferPointer`s
- Swift 3 Named protocols.  Swift 3 naming conventions suggest the elimination of redundant words that can be inferred from variable/function signatures.  This renaming is applied throughout the Swift 3 library converting most naming conventions used in the Swift2/Cocoa library to Swift 3-esque naming conventions. eg.
```swift
func readString() throws -> String
func writeString(_ val: String) throws
```
have been renamed to eliminate redundant words:
```swift
func read() throws -> String
func write(_ val: String) throws
```

- Eliminated `THTTPTransport` that uses `NSURLConnection` due to it being deprecated and not available at all in Swift 3 for Linux.  `THTTPSessionTransport` from the Swift2/Cocoa library that uses `NSURLSession` has been renamed to `THTTPTransport` for this library and leverages `URLSession`, providing both synchronous (with semaphores) and asynchronous behavior.
- Probably some More things I've missed here.

## Generator Notes
#### Generator Flags
| Flag          | Description           |
| ------------- |:-------------:|
| async_clients | Generate clients which invoke asynchronously via block syntax.Asynchronous classes are appended with `_Async` |
| no_strict*    | Generates non-strict structs      |
| debug_descriptions | Allow use of debugDescription so the app can add description via a cateogory/extension      |
| log_unexpected | Log every time an unexpected field ID or type is encountered. |



*Most thrift libraries allow empty initialization of Structs, initializing `required` fields with nil/null/None (Python and Node generators).  Swift on the other hand requires initializers to initialize all non-Optional fields, and thus the Swift 3 generator does not provide default values (unlike the Swift 2/Cocoa generator).  In other languages, this allows the sending of NULL values in fields that are marked `required`, and thus will throw an error in Swift clients attempting to validate fields.  The `no_strict` option here will ignore the validation check, as well as behave similar to the Swift2/Cocoa generator and initialize required fields with empty initializers (where possible).


## Whats implemented
#### Library
##### Transports
- [x] TSocketTransport - CFSocket and PosixSocket variants available. CFSocket variant only currently available for Darwin platforms
- [x] THTTPTransport - Currently only available for Darwin platforms, Swift Foundation URLSession implementation needs completion on linux.
- [x] TSocketServer - Uses CFSockets only for binding, should be working on linux
- [x] TFramedTransport
- [x] TMemoryBufferTransport
- [x] TFileTransport - A few variants using File handles and file descriptors.
- [x] TStreamTransport - Fully functional in Darwin, Foundation backing not yet completed in Linux (This limits TCFSocketTransport to Darwin)
- [ ] HTTPServer - Currently there is no lightweight  HTTPServer implementation the Swift Standard Library, so other 3rd party alternatives are required and out of scope for the Thrift library.  Examples using Perfect will be provided.
- [ ] Other (gz, etc)

##### Protocols
- [x] TBinaryProtocol
- [x] TCompactProtocol
- [ ] TJSONProtocol - This will need to be implemented

##### Generator
- [x] Code Complete Generator
- [x] Async clients
- [x] Documentation Generation - Generator will transplant IDL docs to Swift code for easy lookup in Xcode
- [ ] Default Values - TODO
- [ ] no_strict mode - TODO
- [ ] Namespacing - Still haven't nailed down a good paradigm for namespacing.  It will likely involve creating subdirectories for different namespaces and expecting the developer to import each subdirectory as separate modules.  It could extend to creating SPM Package manifests with sub-modules within the generated module



## Example HTTP Server with Perfect
```swift
import PerfectLib
import PerfectHTTP
import PerfectHTTPServer
import Dispatch

let logQueue = DispatchQueue(label: "log", qos: .background, attributes: .concurrent)
let pQueue = DispatchQueue(label: "log", qos: .userInitiated, attributes: .concurrent)


class TPerfectServer<InProtocol: TProtocol, OutProtocol: TProtocol> {

 private var server = HTTPServer()
 private var processor: TProcessor

 init(address: String? = nil,
      path: String? = nil,
      port: Int,
      processor: TProcessor,
      inProtocol: InProtocol.Type,
      outProtocol: OutProtocol.Type) throws {

   self.processor = processor

   if let address = address {
     server.serverAddress = address
   }
   server.serverPort = UInt16(port)

   var routes = Routes()
   var uri = "/"
   if let path = path {
     uri += path
   }
   routes.add(method: .post, uri: uri) { request, response in
     pQueue.async {
       response.setHeader(.contentType, value: "application/x-thrift")

       let itrans = TMemoryBufferTransport()
       if let bytes = request.postBodyBytes {
         let data = Data(bytes: bytes)
         itrans.reset(readBuffer: data)
       }

       let otrans = TMemoryBufferTransport(flushHandler: { trans, buff in
         let array = buff.withUnsafeBytes {
           Array<UInt8>(UnsafeBufferPointer(start: $0, count: buff.count))
         }
         response.status = .ok
         response.setBody(bytes: array)
         response.completed()
       })

       let inproto = InProtocol(on: itrans)
       let outproto = OutProtocol(on: otrans)

       do {
         try processor.process(on: inproto, outProtocol: outproto)
         try otrans.flush()
       } catch {
         response.status = .badRequest
         response.completed()
       }
     }
   }
   server.addRoutes(routes)
 }

 func serve() throws {
   try server.start()
 }
}
```

#### Example Usage
```swift
class ServiceHandler : Service {
    ...
}
let server = try? TPerfectServer(port: 9090,
                                processor: ServiceProcessor(service: ServiceHandler()),
                                inProtocol: TBinaryProtocol.self,
                                outProtocol: TBinaryProtocol.self)

try? server?.serve()
```
