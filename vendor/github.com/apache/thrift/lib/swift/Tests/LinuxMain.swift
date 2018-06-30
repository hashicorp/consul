import XCTest
@testable import ThriftTests

XCTMain([
     testCase(ThriftTests.allTests),
     testCase(TBinaryProtocolTests.allTests),
     testCase(TCompactProtocolTests.allTests),
])
