//
//  TBinaryProtocolTests.swift
//  Thrift
//
//  Created by Christopher Simpson on 8/18/16.
//
//

import XCTest
import Foundation
@testable import Thrift


/// Testing Binary protocol read/write against itself
/// Uses separate read/write transport/protocols 
class TBinaryProtocolTests: XCTestCase {
  var transport: TMemoryBufferTransport = TMemoryBufferTransport(flushHandler: {
    $0.reset(readBuffer: $1)
  })
  
  var proto: TBinaryProtocol!
  
  override func setUp() {
    super.setUp()
    proto = TBinaryProtocol(on: transport)
    transport.reset()
  }
  
  override func tearDown() {
    super.tearDown()
    transport.reset()
  }
  
  func testInt8WriteRead() {
    let writeVal: UInt8 = 250
    try? proto.write(writeVal)
    try? transport.flush()

    let readVal: UInt8 = (try? proto.read()) ?? 0
    XCTAssertEqual(writeVal, readVal, "Error with UInt8, wrote \(writeVal) but read \(readVal)")
  }
  
  func testInt16WriteRead() {

    let writeVal: Int16 = 12312
    try? proto.write(writeVal)
    try? transport.flush()
    let readVal: Int16 = (try? proto.read()) ?? 0
    XCTAssertEqual(writeVal, readVal, "Error with Int16, wrote \(writeVal) but read \(readVal)")
  }
  
  func testInt32WriteRead() {
    let writeVal: Int32 = 2029234
    try? proto.write(writeVal)
    try? transport.flush()

    let readVal: Int32 = (try? proto.read()) ?? 0
    XCTAssertEqual(writeVal, readVal, "Error with Int32, wrote \(writeVal) but read \(readVal)")
  }
  
  func testInt64WriteRead() {
    let writeVal: Int64 = 234234981374134
    try? proto.write(writeVal)
    try? transport.flush()

    let readVal: Int64 = (try? proto.read()) ?? 0
    XCTAssertEqual(writeVal, readVal, "Error with Int64, wrote \(writeVal) but read \(readVal)")
  }
  
  func testDoubleWriteRead() {
    let writeVal: Double = 3.1415926
    try? proto.write(writeVal)
    try? transport.flush()

    let readVal: Double = (try? proto.read()) ?? 0.0
    XCTAssertEqual(writeVal, readVal, "Error with Double, wrote \(writeVal) but read \(readVal)")
  }
  
  func testBoolWriteRead() {
    let writeVal: Bool = true
    try? proto.write(writeVal)
    try? transport.flush()

    let readVal: Bool = (try? proto.read()) ?? false
    XCTAssertEqual(writeVal, readVal, "Error with Bool, wrote \(writeVal) but read \(readVal)")
  }
  
  func testStringWriteRead() {
    let writeVal: String = "Hello World"
    try? proto.write(writeVal)
    try? transport.flush()

    let readVal: String!
    do {
      readVal = try proto.read()
    } catch let error {
      XCTAssertFalse(true, "Error reading \(error)")
      return
    }
    
    XCTAssertEqual(writeVal, readVal, "Error with String, wrote \(writeVal) but read \(readVal)")
  }
  
  func testDataWriteRead() {
    let writeVal: Data = "Data World".data(using: .utf8)!
    try? proto.write(writeVal)
    try? transport.flush()

    let readVal: Data = (try? proto.read()) ?? "Goodbye World".data(using: .utf8)!
    XCTAssertEqual(writeVal, readVal, "Error with Data, wrote \(writeVal) but read \(readVal)")
  }
  
  func testStructWriteRead() {
    let msg = "Test Protocol Error"
    let writeVal = TApplicationError(error: .protocolError, message: msg)
    do {
      try writeVal.write(to: proto)
      try? transport.flush()

    } catch let error {
      XCTAssertFalse(true, "Caught Error attempting to write \(error)")
    }
    
    do {
      let readVal = try TApplicationError.read(from: proto)
      XCTAssertEqual(readVal.error.thriftErrorCode, writeVal.error.thriftErrorCode, "Error case mismatch, expected \(readVal.error) got \(writeVal.error)")
      XCTAssertEqual(readVal.message, writeVal.message, "Error message mismatch, expected \(readVal.message) got \(writeVal.message)")
    } catch let error {
      XCTAssertFalse(true, "Caught Error attempting to read \(error)")
    }
  }
  
  static var allTests : [(String, (TBinaryProtocolTests) -> () throws -> Void)] {
    return [
      ("testInt8WriteRead", testInt8WriteRead),
      ("testInt16WriteRead", testInt16WriteRead),
      ("testInt32WriteRead", testInt32WriteRead),
      ("testInt64WriteRead", testInt64WriteRead),
      ("testDoubleWriteRead", testDoubleWriteRead),
      ("testBoolWriteRead", testBoolWriteRead),
      ("testStringWriteRead", testStringWriteRead),
      ("testDataWriteRead", testDataWriteRead),
      ("testStructWriteRead", testStructWriteRead)
    ]
  }
}
