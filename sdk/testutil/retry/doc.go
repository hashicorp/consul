// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

// Package retry provides support for repeating operations in tests.
//
// A sample retry operation looks like this:
//
//	func TestX(t *testing.T) {
//	    retry.Run(t, func(r *retry.R) {
//	        if err := foo(); err != nil {
//				r.Errorf("foo: %s", err)
//				return
//	        }
//	    })
//	}
//
// Run uses the DefaultFailer, which is a Timer with a Timeout of 7s,
// and a Wait of 25ms. To customize, use RunWith.
//
// WARNING: unlike *testing.T, *retry.R#Fatal and FailNow *do not*
// fail the test function entirely, only the current run the retry func
package retry
