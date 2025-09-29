When reviewing HashiCorp Consul pull requests, enforce these specific requirements:

LANGUAGE AND DOCUMENTATION
1. All comments must be grammatically correct with proper punctuation and capitalization
2. Changelog entries and other text entries like documentation changes must follow proper grammar and style guidelines
3. Error messages should be user-facing and grammatically correct
4. Function/method documentation must follow Go's godoc format exactly
5. Variable and function names must use proper camelCase/PascalCase conventions
6. No spelling errors in comments, documentation, or string literals

GO CODE QUALITY
7. All functions must handle errors explicitly - no ignored errors with underscore
8. Use `fmt.Errorf` for error wrapping, not string concatenation
9. Context must be first parameter in all functions that accept it
10. Interfaces should be defined where they are used, not where they are implemented
11. Struct initialization must use explicit field names for structs with more than 3 fields
12. No naked returns in functions longer than 5 lines
13. Constants must be grouped in const blocks with proper iota usage where applicable
14. All exported functions/types must have comments starting with the function/type name
15. Use early returns to reduce nesting levels
16. Prefer composition over inheritance - use embedding correctly

IMPORTS AND DEPENDENCIES
17. Import groups must follow this order: stdlib, external, consul internal
20. No unused imports or variables
21. Vendor dependencies should not be modified directly

TESTING REQUIREMENTS
22. Every new function must have corresponding unit tests
23. Use `testify/require` for assertions.
25. Use `testify/suite` for complex test setups
27. No hardcoded sleep statements in tests - use proper synchronization
28. Mock objects must be from `grpcmocks/` package
29. Test setup must use `testutil.TestContext()` for context

CONSUL-SPECIFIC PATTERNS
30. ACL checks must be performed before any resource operations
31. HTTP endpoints must validate input parameters and return consistent error formats
32. Service mesh code must handle Envoy proxy lifecycle correctly
33. All configuration must use HCL format validation
34. Agent configuration must follow existing patterns in agent/config
35. RPC endpoints must properly handle datacenter routing and forwarding

SECURITY ENFORCEMENT
36. Never log sensitive information like tokens, passwords, or private keys
37. All user inputs must be validated and sanitized
38. ACL permissions must follow least privilege principle
39. TLS configuration must use modern cipher suites

PERFORMANCE REQUIREMENTS
42. Implement proper caching with TTL where appropriate
43. Avoid memory leaks - properly close channels, connections, and file handles
44. Use connection pooling for network operations

ERROR HANDLING
50. Errors must be wrapped with context using `fmt.Errorf` with %w verb
51. Log appropriate error levels using `go-hclog` (Debug, Info, Warn, Error)
52. Panic should only be used for truly unrecoverable errors
53. Error messages must be actionable and include relevant context

CONCURRENCY AND RACE CONDITIONS
54. All shared state must be protected with proper synchronization
55. Use channels for communication, mutexes for protecting state
56. Avoid goroutine leaks by ensuring proper cleanup
57. Context cancellation must be respected in all blocking operations

BREAKING CHANGES
58. API changes must maintain backward compatibility or be clearly documented
59. Configuration changes require migration documentation
61. Deprecation warnings must be added before removing functionality

LOGICAL ERROR DETECTION AND PREVENTION
62. Check for off-by-one errors in loops, array indexing, and boundary conditions
63. Verify nil pointer checks are present before dereferencing pointers or interfaces
64. Ensure proper initialization of maps, slices, and channels before use
65. Validate that mutex locks have corresponding unlocks in all code paths
66. Check for resource leaks - files, connections, goroutines must be properly closed/stopped
67. Verify error handling covers all failure scenarios, not just happy path
68. Ensure atomic operations are used correctly for shared counters and flags
69. Check for race conditions in concurrent access to shared data structures
70. Validate that timeouts and contexts are properly propagated and respected
71. Ensure cleanup functions (defer statements) handle errors appropriately
72. Check for infinite loops or recursion without proper exit conditions
73. Verify that slice/map bounds are checked before access to prevent panics
74. Ensure proper handling of zero values and empty collections
75. Check that string operations handle UTF-8 encoding correctly
76. Validate that network operations handle connection failures and retries properly
77. Ensure proper validation of user input ranges and formats before processing
78. Check for inconsistent state updates that could lead to data corruption
79. Verify that error paths properly clean up partial operations
80. Ensure proper handling of edge cases like empty inputs, large datasets, or malformed data
81. Check for logical inconsistencies in conditional statements and boolean expressions
82. Validate that async operations properly handle cancellation and cleanup
83. Ensure proper ordering of operations when dealing with dependencies
84. Check for potential deadlocks in multi-lock scenarios
85. Verify that retry logic includes exponential backoff and maximum retry limits