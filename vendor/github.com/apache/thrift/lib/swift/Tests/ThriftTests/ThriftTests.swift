import XCTest
@testable import Thrift

class ThriftTests: XCTestCase {
  func testVersion() {
    XCTAssertEqual(Thrift().version, "0.0.1")
  }
  
  func test_in_addr_extension() {
    
  }
  
  static var allTests : [(String, (ThriftTests) -> () throws -> Void)] {
    return [
      ("testVersion", testVersion),
    ]
  }
}
