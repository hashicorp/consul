// BSON library for Go
//
// Copyright (c) 2010-2012 - Gustavo Niemeyer <gustavo@niemeyer.net>
//
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are met:
//
// 1. Redistributions of source code must retain the above copyright notice, this
//    list of conditions and the following disclaimer.
// 2. Redistributions in binary form must reproduce the above copyright notice,
//    this list of conditions and the following disclaimer in the documentation
//    and/or other materials provided with the distribution.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
// ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
// WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
// DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT OWNER OR CONTRIBUTORS BE LIABLE FOR
// ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
// (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES;
// LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND
// ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
// SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package bson_test

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/mgo.v2/bson"

	. "gopkg.in/check.v1"
)

// --------------------------------------------------------------------------
// Decimal tests

type decimalTests struct {
	Valid []struct {
		Description      string `json:"description"`
		BSON             string `json:"bson"`
		CanonicalBSON    string `json:"canonical_bson"`
		ExtJSON          string `json:"extjson"`
		CanonicalExtJSON string `json:"canonical_extjson"`
		Lossy            bool   `json:"lossy"`
	} `json:"valid"`

	ParseErrors []struct {
		Description string `json:"description"`
		String      string `json:"string"`
	} `json:"parseErrors"`
}

func extJSONRepr(s string) string {
	var value struct {
		D struct {
			Repr string `json:"$numberDecimal"`
		} `json:"d"`
	}
	err := json.Unmarshal([]byte(s), &value)
	if err != nil {
		panic(err)
	}
	return value.D.Repr
}

func (s *S) TestDecimalTests(c *C) {
	// These also conform to the spec and are used by Go elsewhere.
	// (e.g. math/big won't parse "Infinity").
	goStr := func(s string) string {
		switch s {
		case "Infinity":
			return "Inf"
		case "-Infinity":
			return "-Inf"
		}
		return s
	}

	for _, testEntry := range decimalTestsJSON {
		testFile := testEntry.file

		var tests decimalTests
		err := json.Unmarshal([]byte(testEntry.json), &tests)
		c.Assert(err, IsNil)

		for _, test := range tests.Valid {
			c.Logf("Running %s test: %s", testFile, test.Description)

			test.BSON = strings.ToLower(test.BSON)

			// Unmarshal value from BSON data.
			bsonData, err := hex.DecodeString(test.BSON)
			var bsonValue struct{ D interface{} }
			err = bson.Unmarshal(bsonData, &bsonValue)
			c.Assert(err, IsNil)
			dec128, ok := bsonValue.D.(bson.Decimal128)
			c.Assert(ok, Equals, true)

			// Extract ExtJSON representations (canonical and not).
			extjRepr := extJSONRepr(test.ExtJSON)
			cextjRepr := extjRepr
			if test.CanonicalExtJSON != "" {
				cextjRepr = extJSONRepr(test.CanonicalExtJSON)
			}

			wantRepr := goStr(cextjRepr)

			// Generate canonical representation.
			c.Assert(dec128.String(), Equals, wantRepr)

			// Parse original canonical representation.
			parsed, err := bson.ParseDecimal128(cextjRepr)
			c.Assert(err, IsNil)
			c.Assert(parsed.String(), Equals, wantRepr)

			// Parse non-canonical representation.
			parsed, err = bson.ParseDecimal128(extjRepr)
			c.Assert(err, IsNil)
			c.Assert(parsed.String(), Equals, wantRepr)

			// Parse Go canonical representation (Inf vs. Infinity).
			parsed, err = bson.ParseDecimal128(wantRepr)
			c.Assert(err, IsNil)
			c.Assert(parsed.String(), Equals, wantRepr)

			// Marshal original value back into BSON data.
			data, err := bson.Marshal(bsonValue)
			c.Assert(err, IsNil)
			c.Assert(hex.EncodeToString(data), Equals, test.BSON)

			if test.Lossy {
				continue
			}

			// Marshal the parsed canonical representation.
			var parsedValue struct{ D interface{} }
			parsedValue.D = parsed
			data, err = bson.Marshal(parsedValue)
			c.Assert(err, IsNil)
			c.Assert(hex.EncodeToString(data), Equals, test.BSON)
		}

		for _, test := range tests.ParseErrors {
			c.Logf("Running %s parse error test: %s (string %q)", testFile, test.Description, test.String)

			_, err := bson.ParseDecimal128(test.String)
			quoted := regexp.QuoteMeta(fmt.Sprintf("%q", test.String))
			c.Assert(err, ErrorMatches, `cannot parse `+quoted+` as a decimal128`)
		}
	}
}

const decBenchNum = "9.999999999999999999999999999999999E+6144"

func (s *S) BenchmarkDecimal128String(c *C) {
	d, err := bson.ParseDecimal128(decBenchNum)
	c.Assert(err, IsNil)
	c.Assert(d.String(), Equals, decBenchNum)

	c.ResetTimer()
	for i := 0; i < c.N; i++ {
		d.String()
	}
}

func (s *S) BenchmarkDecimal128Parse(c *C) {
	var err error
	c.ResetTimer()
	for i := 0; i < c.N; i++ {
		_, err = bson.ParseDecimal128(decBenchNum)
	}
	if err != nil {
		panic(err)
	}
}

var decimalTestsJSON = []struct{ file, json string }{
	{"decimal128-1.json", `
{
    "description": "Decimal128",
    "bson_type": "0x13",
    "test_key": "d",
    "valid": [
        {
            "description": "Special - Canonical NaN",
            "bson": "180000001364000000000000000000000000000000007C00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"NaN\"}}"
        },
        {
            "description": "Special - Negative NaN",
            "bson": "18000000136400000000000000000000000000000000FC00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"NaN\"}}",
            "lossy": true
        },
        {
            "description": "Special - Negative NaN",
            "bson": "18000000136400000000000000000000000000000000FC00",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"NaN\"}}",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"-NaN\"}}",
            "lossy": true
        },
        {
            "description": "Special - Canonical SNaN",
            "bson": "180000001364000000000000000000000000000000007E00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"NaN\"}}",
            "lossy": true
        },
        {
            "description": "Special - Negative SNaN",
            "bson": "18000000136400000000000000000000000000000000FE00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"NaN\"}}",
            "lossy": true
        },
        {
            "description": "Special - NaN with a payload",
            "bson": "180000001364001200000000000000000000000000007E00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"NaN\"}}",
            "lossy": true
        },
        {
            "description": "Special - Canonical Positive Infinity",
            "bson": "180000001364000000000000000000000000000000007800",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"Infinity\"}}"
        },
        {
            "description": "Special - Canonical Negative Infinity",
            "bson": "18000000136400000000000000000000000000000000F800",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"-Infinity\"}}"
        },
        {
            "description": "Special - Invalid representation treated as 0",
            "bson": "180000001364000000000000000000000000000000106C00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"0\"}}",
            "lossy": true
        },
        {
            "description": "Special - Invalid representation treated as -0",
            "bson": "18000000136400DCBA9876543210DEADBEEF00000010EC00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0\"}}",
            "lossy": true
        },
        {
            "description": "Special - Invalid representation treated as 0E3",
            "bson": "18000000136400FFFFFFFFFFFFFFFFFFFFFFFFFFFF116C00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"0E+3\"}}",
            "lossy": true
        },
        {
            "description": "Regular - Adjusted Exponent Limit",
            "bson": "18000000136400F2AF967ED05C82DE3297FF6FDE3CF22F00",
            "extjson": "{\"d\": { \"$numberDecimal\": \"0.000001234567890123456789012345678901234\" }}"
        },
        {
            "description": "Regular - Smallest",
            "bson": "18000000136400D204000000000000000000000000343000",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.001234\"}}"
        },
        {
            "description": "Regular - Smallest with Trailing Zeros",
            "bson": "1800000013640040EF5A07000000000000000000002A3000",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.00123400000\"}}"
        },
        {
            "description": "Regular - 0.1",
            "bson": "1800000013640001000000000000000000000000003E3000",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.1\"}}"
        },
        {
            "description": "Regular - 0.1234567890123456789012345678901234",
            "bson": "18000000136400F2AF967ED05C82DE3297FF6FDE3CFC2F00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.1234567890123456789012345678901234\"}}"
        },
        {
            "description": "Regular - 0",
            "bson": "180000001364000000000000000000000000000000403000",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"0\"}}"
        },
        {
            "description": "Regular - -0",
            "bson": "18000000136400000000000000000000000000000040B000",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0\"}}"
        },
        {
            "description": "Regular - -0.0",
            "bson": "1800000013640000000000000000000000000000003EB000",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0.0\"}}"
        },
        {
            "description": "Regular - 2",
            "bson": "180000001364000200000000000000000000000000403000",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"2\"}}"
        },
        {
            "description": "Regular - 2.000",
            "bson": "18000000136400D0070000000000000000000000003A3000",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"2.000\"}}"
        },
        {
            "description": "Regular - Largest",
            "bson": "18000000136400F2AF967ED05C82DE3297FF6FDE3C403000",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1234567890123456789012345678901234\"}}"
        },
        {
            "description": "Scientific - Tiniest",
            "bson": "18000000136400FFFFFFFF638E8D37C087ADBE09ED010000",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"9.999999999999999999999999999999999E-6143\"}}"
        },
        {
            "description": "Scientific - Tiny",
            "bson": "180000001364000100000000000000000000000000000000",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E-6176\"}}"
        },
        {
            "description": "Scientific - Negative Tiny",
            "bson": "180000001364000100000000000000000000000000008000",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"-1E-6176\"}}"
        },
        {
            "description": "Scientific - Adjusted Exponent Limit",
            "bson": "18000000136400F2AF967ED05C82DE3297FF6FDE3CF02F00",
            "extjson": "{\"d\": { \"$numberDecimal\": \"1.234567890123456789012345678901234E-7\" }}"
        },
        {
            "description": "Scientific - Fractional",
            "bson": "1800000013640064000000000000000000000000002CB000",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"-1.00E-8\"}}"
        },
        {
            "description": "Scientific - 0 with Exponent",
            "bson": "180000001364000000000000000000000000000000205F00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"0E+6000\"}}"
        },
        {
            "description": "Scientific - 0 with Negative Exponent",
            "bson": "1800000013640000000000000000000000000000007A2B00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"0E-611\"}}"
        },
        {
            "description": "Scientific - No Decimal with Signed Exponent",
            "bson": "180000001364000100000000000000000000000000463000",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+3\"}}"
        },
        {
            "description": "Scientific - Trailing Zero",
            "bson": "180000001364001A04000000000000000000000000423000",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.050E+4\"}}"
        },
        {
            "description": "Scientific - With Decimal",
            "bson": "180000001364006900000000000000000000000000423000",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.05E+3\"}}"
        },
        {
            "description": "Scientific - Full",
            "bson": "18000000136400FFFFFFFFFFFFFFFFFFFFFFFFFFFF403000",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"5192296858534827628530496329220095\"}}"
        },
        {
            "description": "Scientific - Large",
            "bson": "18000000136400000000000A5BC138938D44C64D31FE5F00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.000000000000000000000000000000000E+6144\"}}"
        },
        {
            "description": "Scientific - Largest",
            "bson": "18000000136400FFFFFFFF638E8D37C087ADBE09EDFF5F00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"9.999999999999999999999999999999999E+6144\"}}"
        },
        {
            "description": "Non-Canonical Parsing - Exponent Normalization",
            "bson": "1800000013640064000000000000000000000000002CB000",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"-100E-10\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"-1.00E-8\"}}"
        },
        {
            "description": "Non-Canonical Parsing - Unsigned Positive Exponent",
            "bson": "180000001364000100000000000000000000000000463000",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E3\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+3\"}}"
        },
        {
            "description": "Non-Canonical Parsing - Lowercase Exponent Identifier",
            "bson": "180000001364000100000000000000000000000000463000",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1e+3\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+3\"}}"
        },
        {
            "description": "Non-Canonical Parsing - Long Significand with Exponent",
            "bson": "1800000013640079D9E0F9763ADA429D0200000000583000",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"12345689012345789012345E+12\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.2345689012345789012345E+34\"}}"
        },
        {
            "description": "Non-Canonical Parsing - Positive Sign",
            "bson": "18000000136400F2AF967ED05C82DE3297FF6FDE3C403000",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"+1234567890123456789012345678901234\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1234567890123456789012345678901234\"}}"
        },
        {
            "description": "Non-Canonical Parsing - Long Decimal String",
            "bson": "180000001364000100000000000000000000000000722800",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \".000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1E-999\"}}"
        },
        {
            "description": "Non-Canonical Parsing - nan",
            "bson": "180000001364000000000000000000000000000000007C00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"nan\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"NaN\"}}"
        },
        {
            "description": "Non-Canonical Parsing - nAn",
            "bson": "180000001364000000000000000000000000000000007C00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"nAn\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"NaN\"}}"
        },
        {
            "description": "Non-Canonical Parsing - +infinity",
            "bson": "180000001364000000000000000000000000000000007800",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"+infinity\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"Infinity\"}}"
        },
        {
            "description": "Non-Canonical Parsing - infinity",
            "bson": "180000001364000000000000000000000000000000007800",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"infinity\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"Infinity\"}}"
        },
        {
            "description": "Non-Canonical Parsing - infiniTY",
            "bson": "180000001364000000000000000000000000000000007800",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"infiniTY\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"Infinity\"}}"
        },
        {
            "description": "Non-Canonical Parsing - inf",
            "bson": "180000001364000000000000000000000000000000007800",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"inf\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"Infinity\"}}"
        },
        {
            "description": "Non-Canonical Parsing - inF",
            "bson": "180000001364000000000000000000000000000000007800",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"inF\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"Infinity\"}}"
        },
        {
            "description": "Non-Canonical Parsing - -infinity",
            "bson": "18000000136400000000000000000000000000000000F800",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"-infinity\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"-Infinity\"}}"
        },
        {
            "description": "Non-Canonical Parsing - -infiniTy",
            "bson": "18000000136400000000000000000000000000000000F800",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"-infiniTy\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"-Infinity\"}}"
        },
        {
            "description": "Non-Canonical Parsing - -Inf",
            "bson": "18000000136400000000000000000000000000000000F800",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"-Infinity\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"-Infinity\"}}"
        },
        {
            "description": "Non-Canonical Parsing - -inf",
            "bson": "18000000136400000000000000000000000000000000F800",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"-inf\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"-Infinity\"}}"
        },
        {
            "description": "Non-Canonical Parsing - -inF",
            "bson": "18000000136400000000000000000000000000000000F800",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"-inF\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"-Infinity\"}}"
        },
        {
           "description": "Rounded Subnormal number",
           "bson": "180000001364000100000000000000000000000000000000",
           "extjson": "{\"d\" : {\"$numberDecimal\" : \"10E-6177\"}}",
           "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1E-6176\"}}"
        },
        {
           "description": "Clamped",
           "bson": "180000001364000a00000000000000000000000000fe5f00",
           "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E6112\"}}",
           "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0E+6112\"}}"
        },
        {
           "description": "Exact rounding",
           "bson": "18000000136400000000000a5bc138938d44c64d31cc3700",
           "extjson": "{\"d\" : {\"$numberDecimal\" : \"1000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000\"}}",
           "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.000000000000000000000000000000000E+999\"}}"
        }
    ]
}
`},

	{"decimal128-2.json", `
{
    "description": "Decimal128",
    "bson_type": "0x13",
    "test_key": "d",
    "valid": [
       {
          "description": "[decq021] Normality",
          "bson": "18000000136400F2AF967ED05C82DE3297FF6FDE3C40B000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-1234567890123456789012345678901234\"}}"
       },
       {
          "description": "[decq823] values around [u]int32 edges (zeros done earlier)",
          "bson": "18000000136400010000800000000000000000000040B000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-2147483649\"}}"
       },
       {
          "description": "[decq822] values around [u]int32 edges (zeros done earlier)",
          "bson": "18000000136400000000800000000000000000000040B000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-2147483648\"}}"
       },
       {
          "description": "[decq821] values around [u]int32 edges (zeros done earlier)",
          "bson": "18000000136400FFFFFF7F0000000000000000000040B000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-2147483647\"}}"
       },
       {
          "description": "[decq820] values around [u]int32 edges (zeros done earlier)",
          "bson": "18000000136400FEFFFF7F0000000000000000000040B000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-2147483646\"}}"
       },
       {
          "description": "[decq152] fold-downs (more below)",
          "bson": "18000000136400393000000000000000000000000040B000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-12345\"}}"
       },
       {
          "description": "[decq154] fold-downs (more below)",
          "bson": "18000000136400D20400000000000000000000000040B000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-1234\"}}"
       },
       {
          "description": "[decq006] derivative canonical plain strings",
          "bson": "18000000136400EE0200000000000000000000000040B000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-750\"}}"
       },
       {
          "description": "[decq164] fold-downs (more below)",
          "bson": "1800000013640039300000000000000000000000003CB000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-123.45\"}}"
       },
       {
          "description": "[decq156] fold-downs (more below)",
          "bson": "180000001364007B0000000000000000000000000040B000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-123\"}}"
       },
       {
          "description": "[decq008] derivative canonical plain strings",
          "bson": "18000000136400EE020000000000000000000000003EB000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-75.0\"}}"
       },
       {
          "description": "[decq158] fold-downs (more below)",
          "bson": "180000001364000C0000000000000000000000000040B000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-12\"}}"
       },
       {
          "description": "[decq122] Nmax and similar",
          "bson": "18000000136400FFFFFFFF638E8D37C087ADBE09EDFFDF00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-9.999999999999999999999999999999999E+6144\"}}"
       },
       {
          "description": "[decq002] (mostly derived from the Strawman 4 document and examples)",
          "bson": "18000000136400EE020000000000000000000000003CB000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-7.50\"}}"
       },
       {
          "description": "[decq004] derivative canonical plain strings",
          "bson": "18000000136400EE0200000000000000000000000042B000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-7.50E+3\"}}"
       },
       {
          "description": "[decq018] derivative canonical plain strings",
          "bson": "18000000136400EE020000000000000000000000002EB000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-7.50E-7\"}}"
       },
       {
          "description": "[decq125] Nmax and similar",
          "bson": "18000000136400F2AF967ED05C82DE3297FF6FDE3CFEDF00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-1.234567890123456789012345678901234E+6144\"}}"
       },
       {
          "description": "[decq131] fold-downs (more below)",
          "bson": "18000000136400000000807F1BCF85B27059C8A43CFEDF00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-1.230000000000000000000000000000000E+6144\"}}"
       },
       {
          "description": "[decq162] fold-downs (more below)",
          "bson": "180000001364007B000000000000000000000000003CB000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-1.23\"}}"
       },
       {
          "description": "[decq176] Nmin and below",
          "bson": "18000000136400010000000A5BC138938D44C64D31008000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-1.000000000000000000000000000000001E-6143\"}}"
       },
       {
          "description": "[decq174] Nmin and below",
          "bson": "18000000136400000000000A5BC138938D44C64D31008000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-1.000000000000000000000000000000000E-6143\"}}"
       },
       {
          "description": "[decq133] fold-downs (more below)",
          "bson": "18000000136400000000000A5BC138938D44C64D31FEDF00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-1.000000000000000000000000000000000E+6144\"}}"
       },
       {
          "description": "[decq160] fold-downs (more below)",
          "bson": "18000000136400010000000000000000000000000040B000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-1\"}}"
       },
       {
          "description": "[decq172] Nmin and below",
          "bson": "180000001364000100000000000000000000000000428000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-1E-6143\"}}"
       },
       {
          "description": "[decq010] derivative canonical plain strings",
          "bson": "18000000136400EE020000000000000000000000003AB000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0.750\"}}"
       },
       {
          "description": "[decq012] derivative canonical plain strings",
          "bson": "18000000136400EE0200000000000000000000000038B000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0.0750\"}}"
       },
       {
          "description": "[decq014] derivative canonical plain strings",
          "bson": "18000000136400EE0200000000000000000000000034B000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0.000750\"}}"
       },
       {
          "description": "[decq016] derivative canonical plain strings",
          "bson": "18000000136400EE0200000000000000000000000030B000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0.00000750\"}}"
       },
       {
          "description": "[decq404] zeros",
          "bson": "180000001364000000000000000000000000000000000000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0E-6176\"}}"
       },
       {
          "description": "[decq424] negative zeros",
          "bson": "180000001364000000000000000000000000000000008000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0E-6176\"}}"
       },
       {
          "description": "[decq407] zeros",
          "bson": "1800000013640000000000000000000000000000003C3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.00\"}}"
       },
       {
          "description": "[decq427] negative zeros",
          "bson": "1800000013640000000000000000000000000000003CB000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0.00\"}}"
       },
       {
          "description": "[decq409] zeros",
          "bson": "180000001364000000000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0\"}}"
       },
       {
          "description": "[decq428] negative zeros",
          "bson": "18000000136400000000000000000000000000000040B000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0\"}}"
       },
       {
          "description": "[decq700] Selected DPD codes",
          "bson": "180000001364000000000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0\"}}"
       },
       {
          "description": "[decq406] zeros",
          "bson": "1800000013640000000000000000000000000000003C3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.00\"}}"
       },
       {
          "description": "[decq426] negative zeros",
          "bson": "1800000013640000000000000000000000000000003CB000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0.00\"}}"
       },
       {
          "description": "[decq410] zeros",
          "bson": "180000001364000000000000000000000000000000463000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0E+3\"}}"
       },
       {
          "description": "[decq431] negative zeros",
          "bson": "18000000136400000000000000000000000000000046B000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0E+3\"}}"
       },
       {
          "description": "[decq419] clamped zeros...",
          "bson": "180000001364000000000000000000000000000000FE5F00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0E+6111\"}}"
       },
       {
          "description": "[decq432] negative zeros",
          "bson": "180000001364000000000000000000000000000000FEDF00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0E+6111\"}}"
       },
       {
          "description": "[decq405] zeros",
          "bson": "180000001364000000000000000000000000000000000000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0E-6176\"}}"
       },
       {
          "description": "[decq425] negative zeros",
          "bson": "180000001364000000000000000000000000000000008000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0E-6176\"}}"
       },
       {
          "description": "[decq508] Specials",
          "bson": "180000001364000000000000000000000000000000007800",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"Infinity\"}}"
       },
       {
          "description": "[decq528] Specials",
          "bson": "18000000136400000000000000000000000000000000F800",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-Infinity\"}}"
       },
       {
          "description": "[decq541] Specials",
          "bson": "180000001364000000000000000000000000000000007C00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"NaN\"}}"
       },
       {
          "description": "[decq074] Nmin and below",
          "bson": "18000000136400000000000A5BC138938D44C64D31000000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.000000000000000000000000000000000E-6143\"}}"
       },
       {
          "description": "[decq602] fold-down full sequence",
          "bson": "18000000136400000000000A5BC138938D44C64D31FE5F00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.000000000000000000000000000000000E+6144\"}}"
       },
       {
          "description": "[decq604] fold-down full sequence",
          "bson": "180000001364000000000081EFAC855B416D2DEE04FE5F00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.00000000000000000000000000000000E+6143\"}}"
       },
       {
          "description": "[decq606] fold-down full sequence",
          "bson": "1800000013640000000080264B91C02220BE377E00FE5F00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0000000000000000000000000000000E+6142\"}}"
       },
       {
          "description": "[decq608] fold-down full sequence",
          "bson": "1800000013640000000040EAED7446D09C2C9F0C00FE5F00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.000000000000000000000000000000E+6141\"}}"
       },
       {
          "description": "[decq610] fold-down full sequence",
          "bson": "18000000136400000000A0CA17726DAE0F1E430100FE5F00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.00000000000000000000000000000E+6140\"}}"
       },
       {
          "description": "[decq612] fold-down full sequence",
          "bson": "18000000136400000000106102253E5ECE4F200000FE5F00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0000000000000000000000000000E+6139\"}}"
       },
       {
          "description": "[decq614] fold-down full sequence",
          "bson": "18000000136400000000E83C80D09F3C2E3B030000FE5F00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.000000000000000000000000000E+6138\"}}"
       },
       {
          "description": "[decq616] fold-down full sequence",
          "bson": "18000000136400000000E4D20CC8DCD2B752000000FE5F00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.00000000000000000000000000E+6137\"}}"
       },
       {
          "description": "[decq618] fold-down full sequence",
          "bson": "180000001364000000004A48011416954508000000FE5F00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0000000000000000000000000E+6136\"}}"
       },
       {
          "description": "[decq620] fold-down full sequence",
          "bson": "18000000136400000000A1EDCCCE1BC2D300000000FE5F00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.000000000000000000000000E+6135\"}}"
       },
       {
          "description": "[decq622] fold-down full sequence",
          "bson": "18000000136400000080F64AE1C7022D1500000000FE5F00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.00000000000000000000000E+6134\"}}"
       },
       {
          "description": "[decq624] fold-down full sequence",
          "bson": "18000000136400000040B2BAC9E0191E0200000000FE5F00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0000000000000000000000E+6133\"}}"
       },
       {
          "description": "[decq626] fold-down full sequence",
          "bson": "180000001364000000A0DEC5ADC935360000000000FE5F00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.000000000000000000000E+6132\"}}"
       },
       {
          "description": "[decq628] fold-down full sequence",
          "bson": "18000000136400000010632D5EC76B050000000000FE5F00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.00000000000000000000E+6131\"}}"
       },
       {
          "description": "[decq630] fold-down full sequence",
          "bson": "180000001364000000E8890423C78A000000000000FE5F00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0000000000000000000E+6130\"}}"
       },
       {
          "description": "[decq632] fold-down full sequence",
          "bson": "18000000136400000064A7B3B6E00D000000000000FE5F00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.000000000000000000E+6129\"}}"
       },
       {
          "description": "[decq634] fold-down full sequence",
          "bson": "1800000013640000008A5D78456301000000000000FE5F00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.00000000000000000E+6128\"}}"
       },
       {
          "description": "[decq636] fold-down full sequence",
          "bson": "180000001364000000C16FF2862300000000000000FE5F00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0000000000000000E+6127\"}}"
       },
       {
          "description": "[decq638] fold-down full sequence",
          "bson": "180000001364000080C6A47E8D0300000000000000FE5F00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.000000000000000E+6126\"}}"
       },
       {
          "description": "[decq640] fold-down full sequence",
          "bson": "1800000013640000407A10F35A0000000000000000FE5F00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.00000000000000E+6125\"}}"
       },
       {
          "description": "[decq642] fold-down full sequence",
          "bson": "1800000013640000A0724E18090000000000000000FE5F00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0000000000000E+6124\"}}"
       },
       {
          "description": "[decq644] fold-down full sequence",
          "bson": "180000001364000010A5D4E8000000000000000000FE5F00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.000000000000E+6123\"}}"
       },
       {
          "description": "[decq646] fold-down full sequence",
          "bson": "1800000013640000E8764817000000000000000000FE5F00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.00000000000E+6122\"}}"
       },
       {
          "description": "[decq648] fold-down full sequence",
          "bson": "1800000013640000E40B5402000000000000000000FE5F00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0000000000E+6121\"}}"
       },
       {
          "description": "[decq650] fold-down full sequence",
          "bson": "1800000013640000CA9A3B00000000000000000000FE5F00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.000000000E+6120\"}}"
       },
       {
          "description": "[decq652] fold-down full sequence",
          "bson": "1800000013640000E1F50500000000000000000000FE5F00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.00000000E+6119\"}}"
       },
       {
          "description": "[decq654] fold-down full sequence",
          "bson": "180000001364008096980000000000000000000000FE5F00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0000000E+6118\"}}"
       },
       {
          "description": "[decq656] fold-down full sequence",
          "bson": "1800000013640040420F0000000000000000000000FE5F00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.000000E+6117\"}}"
       },
       {
          "description": "[decq658] fold-down full sequence",
          "bson": "18000000136400A086010000000000000000000000FE5F00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.00000E+6116\"}}"
       },
       {
          "description": "[decq660] fold-down full sequence",
          "bson": "180000001364001027000000000000000000000000FE5F00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0000E+6115\"}}"
       },
       {
          "description": "[decq662] fold-down full sequence",
          "bson": "18000000136400E803000000000000000000000000FE5F00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.000E+6114\"}}"
       },
       {
          "description": "[decq664] fold-down full sequence",
          "bson": "180000001364006400000000000000000000000000FE5F00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.00E+6113\"}}"
       },
       {
          "description": "[decq666] fold-down full sequence",
          "bson": "180000001364000A00000000000000000000000000FE5F00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0E+6112\"}}"
       },
       {
          "description": "[decq060] fold-downs (more below)",
          "bson": "180000001364000100000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1\"}}"
       },
       {
          "description": "[decq670] fold-down full sequence",
          "bson": "180000001364000100000000000000000000000000FC5F00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+6110\"}}"
       },
       {
          "description": "[decq668] fold-down full sequence",
          "bson": "180000001364000100000000000000000000000000FE5F00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+6111\"}}"
       },
       {
          "description": "[decq072] Nmin and below",
          "bson": "180000001364000100000000000000000000000000420000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E-6143\"}}"
       },
       {
          "description": "[decq076] Nmin and below",
          "bson": "18000000136400010000000A5BC138938D44C64D31000000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.000000000000000000000000000000001E-6143\"}}"
       },
       {
          "description": "[decq036] fold-downs (more below)",
          "bson": "18000000136400000000807F1BCF85B27059C8A43CFE5F00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.230000000000000000000000000000000E+6144\"}}"
       },
       {
          "description": "[decq062] fold-downs (more below)",
          "bson": "180000001364007B000000000000000000000000003C3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.23\"}}"
       },
       {
          "description": "[decq034] Nmax and similar",
          "bson": "18000000136400F2AF967ED05C82DE3297FF6FDE3CFE5F00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.234567890123456789012345678901234E+6144\"}}"
       },
       {
          "description": "[decq441] exponent lengths",
          "bson": "180000001364000700000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"7\"}}"
       },
       {
          "description": "[decq449] exponent lengths",
          "bson": "1800000013640007000000000000000000000000001E5F00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"7E+5999\"}}"
       },
       {
          "description": "[decq447] exponent lengths",
          "bson": "1800000013640007000000000000000000000000000E3800",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"7E+999\"}}"
       },
       {
          "description": "[decq445] exponent lengths",
          "bson": "180000001364000700000000000000000000000000063100",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"7E+99\"}}"
       },
       {
          "description": "[decq443] exponent lengths",
          "bson": "180000001364000700000000000000000000000000523000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"7E+9\"}}"
       },
       {
          "description": "[decq842] VG testcase",
          "bson": "180000001364000000FED83F4E7C9FE4E269E38A5BCD1700",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"7.049000000000010795488000000000000E-3097\"}}"
       },
       {
          "description": "[decq841] VG testcase",
          "bson": "180000001364000000203B9DB5056F000000000000002400",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"8.000000000000000000E-1550\"}}"
       },
       {
          "description": "[decq840] VG testcase",
          "bson": "180000001364003C17258419D710C42F0000000000002400",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"8.81125000000001349436E-1548\"}}"
       },
       {
          "description": "[decq701] Selected DPD codes",
          "bson": "180000001364000900000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"9\"}}"
       },
       {
          "description": "[decq032] Nmax and similar",
          "bson": "18000000136400FFFFFFFF638E8D37C087ADBE09EDFF5F00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"9.999999999999999999999999999999999E+6144\"}}"
       },
       {
          "description": "[decq702] Selected DPD codes",
          "bson": "180000001364000A00000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"10\"}}"
       },
       {
          "description": "[decq057] fold-downs (more below)",
          "bson": "180000001364000C00000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"12\"}}"
       },
       {
          "description": "[decq703] Selected DPD codes",
          "bson": "180000001364001300000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"19\"}}"
       },
       {
          "description": "[decq704] Selected DPD codes",
          "bson": "180000001364001400000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"20\"}}"
       },
       {
          "description": "[decq705] Selected DPD codes",
          "bson": "180000001364001D00000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"29\"}}"
       },
       {
          "description": "[decq706] Selected DPD codes",
          "bson": "180000001364001E00000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"30\"}}"
       },
       {
          "description": "[decq707] Selected DPD codes",
          "bson": "180000001364002700000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"39\"}}"
       },
       {
          "description": "[decq708] Selected DPD codes",
          "bson": "180000001364002800000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"40\"}}"
       },
       {
          "description": "[decq709] Selected DPD codes",
          "bson": "180000001364003100000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"49\"}}"
       },
       {
          "description": "[decq710] Selected DPD codes",
          "bson": "180000001364003200000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"50\"}}"
       },
       {
          "description": "[decq711] Selected DPD codes",
          "bson": "180000001364003B00000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"59\"}}"
       },
       {
          "description": "[decq712] Selected DPD codes",
          "bson": "180000001364003C00000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"60\"}}"
       },
       {
          "description": "[decq713] Selected DPD codes",
          "bson": "180000001364004500000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"69\"}}"
       },
       {
          "description": "[decq714] Selected DPD codes",
          "bson": "180000001364004600000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"70\"}}"
       },
       {
          "description": "[decq715] Selected DPD codes",
          "bson": "180000001364004700000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"71\"}}"
       },
       {
          "description": "[decq716] Selected DPD codes",
          "bson": "180000001364004800000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"72\"}}"
       },
       {
          "description": "[decq717] Selected DPD codes",
          "bson": "180000001364004900000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"73\"}}"
       },
       {
          "description": "[decq718] Selected DPD codes",
          "bson": "180000001364004A00000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"74\"}}"
       },
       {
          "description": "[decq719] Selected DPD codes",
          "bson": "180000001364004B00000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"75\"}}"
       },
       {
          "description": "[decq720] Selected DPD codes",
          "bson": "180000001364004C00000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"76\"}}"
       },
       {
          "description": "[decq721] Selected DPD codes",
          "bson": "180000001364004D00000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"77\"}}"
       },
       {
          "description": "[decq722] Selected DPD codes",
          "bson": "180000001364004E00000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"78\"}}"
       },
       {
          "description": "[decq723] Selected DPD codes",
          "bson": "180000001364004F00000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"79\"}}"
       },
       {
          "description": "[decq056] fold-downs (more below)",
          "bson": "180000001364007B00000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"123\"}}"
       },
       {
          "description": "[decq064] fold-downs (more below)",
          "bson": "1800000013640039300000000000000000000000003C3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"123.45\"}}"
       },
       {
          "description": "[decq732] Selected DPD codes",
          "bson": "180000001364000802000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"520\"}}"
       },
       {
          "description": "[decq733] Selected DPD codes",
          "bson": "180000001364000902000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"521\"}}"
       },
       {
          "description": "[decq740] DPD: one of each of the huffman groups",
          "bson": "180000001364000903000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"777\"}}"
       },
       {
          "description": "[decq741] DPD: one of each of the huffman groups",
          "bson": "180000001364000A03000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"778\"}}"
       },
       {
          "description": "[decq742] DPD: one of each of the huffman groups",
          "bson": "180000001364001303000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"787\"}}"
       },
       {
          "description": "[decq746] DPD: one of each of the huffman groups",
          "bson": "180000001364001F03000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"799\"}}"
       },
       {
          "description": "[decq743] DPD: one of each of the huffman groups",
          "bson": "180000001364006D03000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"877\"}}"
       },
       {
          "description": "[decq753] DPD all-highs cases (includes the 24 redundant codes)",
          "bson": "180000001364007803000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"888\"}}"
       },
       {
          "description": "[decq754] DPD all-highs cases (includes the 24 redundant codes)",
          "bson": "180000001364007903000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"889\"}}"
       },
       {
          "description": "[decq760] DPD all-highs cases (includes the 24 redundant codes)",
          "bson": "180000001364008203000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"898\"}}"
       },
       {
          "description": "[decq764] DPD all-highs cases (includes the 24 redundant codes)",
          "bson": "180000001364008303000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"899\"}}"
       },
       {
          "description": "[decq745] DPD: one of each of the huffman groups",
          "bson": "18000000136400D303000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"979\"}}"
       },
       {
          "description": "[decq770] DPD all-highs cases (includes the 24 redundant codes)",
          "bson": "18000000136400DC03000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"988\"}}"
       },
       {
          "description": "[decq774] DPD all-highs cases (includes the 24 redundant codes)",
          "bson": "18000000136400DD03000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"989\"}}"
       },
       {
          "description": "[decq730] Selected DPD codes",
          "bson": "18000000136400E203000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"994\"}}"
       },
       {
          "description": "[decq731] Selected DPD codes",
          "bson": "18000000136400E303000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"995\"}}"
       },
       {
          "description": "[decq744] DPD: one of each of the huffman groups",
          "bson": "18000000136400E503000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"997\"}}"
       },
       {
          "description": "[decq780] DPD all-highs cases (includes the 24 redundant codes)",
          "bson": "18000000136400E603000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"998\"}}"
       },
       {
          "description": "[decq787] DPD all-highs cases (includes the 24 redundant codes)",
          "bson": "18000000136400E703000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"999\"}}"
       },
       {
          "description": "[decq053] fold-downs (more below)",
          "bson": "18000000136400D204000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1234\"}}"
       },
       {
          "description": "[decq052] fold-downs (more below)",
          "bson": "180000001364003930000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"12345\"}}"
       },
       {
          "description": "[decq792] Miscellaneous (testers' queries, etc.)",
          "bson": "180000001364003075000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"30000\"}}"
       },
       {
          "description": "[decq793] Miscellaneous (testers' queries, etc.)",
          "bson": "1800000013640090940D0000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"890000\"}}"
       },
       {
          "description": "[decq824] values around [u]int32 edges (zeros done earlier)",
          "bson": "18000000136400FEFFFF7F00000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"2147483646\"}}"
       },
       {
          "description": "[decq825] values around [u]int32 edges (zeros done earlier)",
          "bson": "18000000136400FFFFFF7F00000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"2147483647\"}}"
       },
       {
          "description": "[decq826] values around [u]int32 edges (zeros done earlier)",
          "bson": "180000001364000000008000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"2147483648\"}}"
       },
       {
          "description": "[decq827] values around [u]int32 edges (zeros done earlier)",
          "bson": "180000001364000100008000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"2147483649\"}}"
       },
       {
          "description": "[decq828] values around [u]int32 edges (zeros done earlier)",
          "bson": "18000000136400FEFFFFFF00000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"4294967294\"}}"
       },
       {
          "description": "[decq829] values around [u]int32 edges (zeros done earlier)",
          "bson": "18000000136400FFFFFFFF00000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"4294967295\"}}"
       },
       {
          "description": "[decq830] values around [u]int32 edges (zeros done earlier)",
          "bson": "180000001364000000000001000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"4294967296\"}}"
       },
       {
          "description": "[decq831] values around [u]int32 edges (zeros done earlier)",
          "bson": "180000001364000100000001000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"4294967297\"}}"
       },
       {
          "description": "[decq022] Normality",
          "bson": "18000000136400C7711CC7B548F377DC80A131C836403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1111111111111111111111111111111111\"}}"
       },
       {
          "description": "[decq020] Normality",
          "bson": "18000000136400F2AF967ED05C82DE3297FF6FDE3C403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1234567890123456789012345678901234\"}}"
       },
       {
          "description": "[decq550] Specials",
          "bson": "18000000136400FFFFFFFF638E8D37C087ADBE09ED413000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"9999999999999999999999999999999999\"}}"
       }
    ]
}
`},

	{"decimal128-3.json", `
{
    "description": "Decimal128",
    "bson_type": "0x13",
    "test_key": "d",
    "valid": [
       {
          "description": "[basx066] strings without E cannot generate E in result",
          "bson": "18000000136400185C0ACE0000000000000000000038B000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-00345678.5432\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"-345678.5432\"}}"
       },
       {
          "description": "[basx065] strings without E cannot generate E in result",
          "bson": "18000000136400185C0ACE0000000000000000000038B000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0345678.5432\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"-345678.5432\"}}"
       },
       {
          "description": "[basx064] strings without E cannot generate E in result",
          "bson": "18000000136400185C0ACE0000000000000000000038B000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-345678.5432\"}}"
       },
       {
          "description": "[basx041] strings without E cannot generate E in result",
          "bson": "180000001364004C0000000000000000000000000040B000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-76\"}}"
       },
       {
          "description": "[basx027] conform to rules and exponent will be in permitted range).",
          "bson": "180000001364000F270000000000000000000000003AB000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-9.999\"}}"
       },
       {
          "description": "[basx026] conform to rules and exponent will be in permitted range).",
          "bson": "180000001364009F230000000000000000000000003AB000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-9.119\"}}"
       },
       {
          "description": "[basx025] conform to rules and exponent will be in permitted range).",
          "bson": "180000001364008F030000000000000000000000003CB000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-9.11\"}}"
       },
       {
          "description": "[basx024] conform to rules and exponent will be in permitted range).",
          "bson": "180000001364005B000000000000000000000000003EB000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-9.1\"}}"
       },
       {
          "description": "[dqbsr531] negatives (Rounded)",
          "bson": "1800000013640099761CC7B548F377DC80A131C836FEAF00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-1.1111111111111111111111111111123450\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"-1.111111111111111111111111111112345\"}}"
       },
       {
          "description": "[basx022] conform to rules and exponent will be in permitted range).",
          "bson": "180000001364000A000000000000000000000000003EB000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-1.0\"}}"
       },
       {
          "description": "[basx021] conform to rules and exponent will be in permitted range).",
          "bson": "18000000136400010000000000000000000000000040B000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-1\"}}"
       },
       {
          "description": "[basx601] Zeros",
          "bson": "1800000013640000000000000000000000000000002E3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.000000000\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0E-9\"}}"
       },
       {
          "description": "[basx622] Zeros",
          "bson": "1800000013640000000000000000000000000000002EB000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0.000000000\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"-0E-9\"}}"
       },
       {
          "description": "[basx602] Zeros",
          "bson": "180000001364000000000000000000000000000000303000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.00000000\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0E-8\"}}"
       },
       {
          "description": "[basx621] Zeros",
          "bson": "18000000136400000000000000000000000000000030B000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0.00000000\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"-0E-8\"}}"
       },
       {
          "description": "[basx603] Zeros",
          "bson": "180000001364000000000000000000000000000000323000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.0000000\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0E-7\"}}"
       },
       {
          "description": "[basx620] Zeros",
          "bson": "18000000136400000000000000000000000000000032B000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0.0000000\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"-0E-7\"}}"
       },
       {
          "description": "[basx604] Zeros",
          "bson": "180000001364000000000000000000000000000000343000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.000000\"}}"
       },
       {
          "description": "[basx619] Zeros",
          "bson": "18000000136400000000000000000000000000000034B000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0.000000\"}}"
       },
       {
          "description": "[basx605] Zeros",
          "bson": "180000001364000000000000000000000000000000363000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.00000\"}}"
       },
       {
          "description": "[basx618] Zeros",
          "bson": "18000000136400000000000000000000000000000036B000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0.00000\"}}"
       },
       {
          "description": "[basx680] Zeros",
          "bson": "180000001364000000000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"000000.\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0\"}}"
       },
       {
          "description": "[basx606] Zeros",
          "bson": "180000001364000000000000000000000000000000383000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.0000\"}}"
       },
       {
          "description": "[basx617] Zeros",
          "bson": "18000000136400000000000000000000000000000038B000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0.0000\"}}"
       },
       {
          "description": "[basx681] Zeros",
          "bson": "180000001364000000000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"00000.\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0\"}}"
       },
       {
          "description": "[basx686] Zeros",
          "bson": "180000001364000000000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"+00000.\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0\"}}"
       },
       {
          "description": "[basx687] Zeros",
          "bson": "18000000136400000000000000000000000000000040B000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-00000.\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"-0\"}}"
       },
       {
          "description": "[basx019] conform to rules and exponent will be in permitted range).",
          "bson": "1800000013640000000000000000000000000000003CB000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-00.00\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"-0.00\"}}"
       },
       {
          "description": "[basx607] Zeros",
          "bson": "1800000013640000000000000000000000000000003A3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.000\"}}"
       },
       {
          "description": "[basx616] Zeros",
          "bson": "1800000013640000000000000000000000000000003AB000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0.000\"}}"
       },
       {
          "description": "[basx682] Zeros",
          "bson": "180000001364000000000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0000.\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0\"}}"
       },
       {
          "description": "[basx155] Numbers with E",
          "bson": "1800000013640000000000000000000000000000003A3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.000e+0\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.000\"}}"
       },
       {
          "description": "[basx130] Numbers with E",
          "bson": "180000001364000000000000000000000000000000383000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.000E-1\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.0000\"}}"
       },
       {
          "description": "[basx290] some more negative zeros [systematic tests below]",
          "bson": "18000000136400000000000000000000000000000038B000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0.000E-1\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"-0.0000\"}}"
       },
       {
          "description": "[basx131] Numbers with E",
          "bson": "180000001364000000000000000000000000000000363000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.000E-2\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.00000\"}}"
       },
       {
          "description": "[basx291] some more negative zeros [systematic tests below]",
          "bson": "18000000136400000000000000000000000000000036B000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0.000E-2\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"-0.00000\"}}"
       },
       {
          "description": "[basx132] Numbers with E",
          "bson": "180000001364000000000000000000000000000000343000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.000E-3\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.000000\"}}"
       },
       {
          "description": "[basx292] some more negative zeros [systematic tests below]",
          "bson": "18000000136400000000000000000000000000000034B000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0.000E-3\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"-0.000000\"}}"
       },
       {
          "description": "[basx133] Numbers with E",
          "bson": "180000001364000000000000000000000000000000323000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.000E-4\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0E-7\"}}"
       },
       {
          "description": "[basx293] some more negative zeros [systematic tests below]",
          "bson": "18000000136400000000000000000000000000000032B000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0.000E-4\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"-0E-7\"}}"
       },
       {
          "description": "[basx608] Zeros",
          "bson": "1800000013640000000000000000000000000000003C3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.00\"}}"
       },
       {
          "description": "[basx615] Zeros",
          "bson": "1800000013640000000000000000000000000000003CB000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0.00\"}}"
       },
       {
          "description": "[basx683] Zeros",
          "bson": "180000001364000000000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"000.\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0\"}}"
       },
       {
          "description": "[basx630] Zeros",
          "bson": "1800000013640000000000000000000000000000003C3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.00E+0\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.00\"}}"
       },
       {
          "description": "[basx670] Zeros",
          "bson": "1800000013640000000000000000000000000000003C3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.00E-0\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.00\"}}"
       },
       {
          "description": "[basx631] Zeros",
          "bson": "1800000013640000000000000000000000000000003E3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.00E+1\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.0\"}}"
       },
       {
          "description": "[basx671] Zeros",
          "bson": "1800000013640000000000000000000000000000003A3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.00E-1\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.000\"}}"
       },
       {
          "description": "[basx134] Numbers with E",
          "bson": "180000001364000000000000000000000000000000383000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.00E-2\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.0000\"}}"
       },
       {
          "description": "[basx294] some more negative zeros [systematic tests below]",
          "bson": "18000000136400000000000000000000000000000038B000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0.00E-2\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"-0.0000\"}}"
       },
       {
          "description": "[basx632] Zeros",
          "bson": "180000001364000000000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.00E+2\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0\"}}"
       },
       {
          "description": "[basx672] Zeros",
          "bson": "180000001364000000000000000000000000000000383000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.00E-2\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.0000\"}}"
       },
       {
          "description": "[basx135] Numbers with E",
          "bson": "180000001364000000000000000000000000000000363000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.00E-3\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.00000\"}}"
       },
       {
          "description": "[basx295] some more negative zeros [systematic tests below]",
          "bson": "18000000136400000000000000000000000000000036B000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0.00E-3\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"-0.00000\"}}"
       },
       {
          "description": "[basx633] Zeros",
          "bson": "180000001364000000000000000000000000000000423000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.00E+3\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0E+1\"}}"
       },
       {
          "description": "[basx673] Zeros",
          "bson": "180000001364000000000000000000000000000000363000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.00E-3\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.00000\"}}"
       },
       {
          "description": "[basx136] Numbers with E",
          "bson": "180000001364000000000000000000000000000000343000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.00E-4\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.000000\"}}"
       },
       {
          "description": "[basx674] Zeros",
          "bson": "180000001364000000000000000000000000000000343000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.00E-4\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.000000\"}}"
       },
       {
          "description": "[basx634] Zeros",
          "bson": "180000001364000000000000000000000000000000443000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.00E+4\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0E+2\"}}"
       },
       {
          "description": "[basx137] Numbers with E",
          "bson": "180000001364000000000000000000000000000000323000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.00E-5\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0E-7\"}}"
       },
       {
          "description": "[basx635] Zeros",
          "bson": "180000001364000000000000000000000000000000463000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.00E+5\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0E+3\"}}"
       },
       {
          "description": "[basx675] Zeros",
          "bson": "180000001364000000000000000000000000000000323000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.00E-5\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0E-7\"}}"
       },
       {
          "description": "[basx636] Zeros",
          "bson": "180000001364000000000000000000000000000000483000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.00E+6\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0E+4\"}}"
       },
       {
          "description": "[basx676] Zeros",
          "bson": "180000001364000000000000000000000000000000303000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.00E-6\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0E-8\"}}"
       },
       {
          "description": "[basx637] Zeros",
          "bson": "1800000013640000000000000000000000000000004A3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.00E+7\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0E+5\"}}"
       },
       {
          "description": "[basx677] Zeros",
          "bson": "1800000013640000000000000000000000000000002E3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.00E-7\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0E-9\"}}"
       },
       {
          "description": "[basx638] Zeros",
          "bson": "1800000013640000000000000000000000000000004C3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.00E+8\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0E+6\"}}"
       },
       {
          "description": "[basx678] Zeros",
          "bson": "1800000013640000000000000000000000000000002C3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.00E-8\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0E-10\"}}"
       },
       {
          "description": "[basx149] Numbers with E",
          "bson": "180000001364000000000000000000000000000000523000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"000E+9\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0E+9\"}}"
       },
       {
          "description": "[basx639] Zeros",
          "bson": "1800000013640000000000000000000000000000004E3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.00E+9\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0E+7\"}}"
       },
       {
          "description": "[basx679] Zeros",
          "bson": "1800000013640000000000000000000000000000002A3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.00E-9\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0E-11\"}}"
       },
       {
          "description": "[basx063] strings without E cannot generate E in result",
          "bson": "18000000136400185C0ACE00000000000000000000383000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"+00345678.5432\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"345678.5432\"}}"
       },
       {
          "description": "[basx018] conform to rules and exponent will be in permitted range).",
          "bson": "1800000013640000000000000000000000000000003EB000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0.0\"}}"
       },
       {
          "description": "[basx609] Zeros",
          "bson": "1800000013640000000000000000000000000000003E3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.0\"}}"
       },
       {
          "description": "[basx614] Zeros",
          "bson": "1800000013640000000000000000000000000000003EB000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0.0\"}}"
       },
       {
          "description": "[basx684] Zeros",
          "bson": "180000001364000000000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"00.\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0\"}}"
       },
       {
          "description": "[basx640] Zeros",
          "bson": "1800000013640000000000000000000000000000003E3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.0E+0\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.0\"}}"
       },
       {
          "description": "[basx660] Zeros",
          "bson": "1800000013640000000000000000000000000000003E3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.0E-0\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.0\"}}"
       },
       {
          "description": "[basx641] Zeros",
          "bson": "180000001364000000000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.0E+1\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0\"}}"
       },
       {
          "description": "[basx661] Zeros",
          "bson": "1800000013640000000000000000000000000000003C3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.0E-1\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.00\"}}"
       },
       {
          "description": "[basx296] some more negative zeros [systematic tests below]",
          "bson": "1800000013640000000000000000000000000000003AB000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0.0E-2\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"-0.000\"}}"
       },
       {
          "description": "[basx642] Zeros",
          "bson": "180000001364000000000000000000000000000000423000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.0E+2\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0E+1\"}}"
       },
       {
          "description": "[basx662] Zeros",
          "bson": "1800000013640000000000000000000000000000003A3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.0E-2\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.000\"}}"
       },
       {
          "description": "[basx297] some more negative zeros [systematic tests below]",
          "bson": "18000000136400000000000000000000000000000038B000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0.0E-3\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"-0.0000\"}}"
       },
       {
          "description": "[basx643] Zeros",
          "bson": "180000001364000000000000000000000000000000443000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.0E+3\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0E+2\"}}"
       },
       {
          "description": "[basx663] Zeros",
          "bson": "180000001364000000000000000000000000000000383000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.0E-3\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.0000\"}}"
       },
       {
          "description": "[basx644] Zeros",
          "bson": "180000001364000000000000000000000000000000463000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.0E+4\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0E+3\"}}"
       },
       {
          "description": "[basx664] Zeros",
          "bson": "180000001364000000000000000000000000000000363000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.0E-4\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.00000\"}}"
       },
       {
          "description": "[basx645] Zeros",
          "bson": "180000001364000000000000000000000000000000483000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.0E+5\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0E+4\"}}"
       },
       {
          "description": "[basx665] Zeros",
          "bson": "180000001364000000000000000000000000000000343000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.0E-5\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.000000\"}}"
       },
       {
          "description": "[basx646] Zeros",
          "bson": "1800000013640000000000000000000000000000004A3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.0E+6\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0E+5\"}}"
       },
       {
          "description": "[basx666] Zeros",
          "bson": "180000001364000000000000000000000000000000323000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.0E-6\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0E-7\"}}"
       },
       {
          "description": "[basx647] Zeros",
          "bson": "1800000013640000000000000000000000000000004C3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.0E+7\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0E+6\"}}"
       },
       {
          "description": "[basx667] Zeros",
          "bson": "180000001364000000000000000000000000000000303000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.0E-7\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0E-8\"}}"
       },
       {
          "description": "[basx648] Zeros",
          "bson": "1800000013640000000000000000000000000000004E3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.0E+8\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0E+7\"}}"
       },
       {
          "description": "[basx668] Zeros",
          "bson": "1800000013640000000000000000000000000000002E3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.0E-8\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0E-9\"}}"
       },
       {
          "description": "[basx160] Numbers with E",
          "bson": "180000001364000000000000000000000000000000523000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"00E+9\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0E+9\"}}"
       },
       {
          "description": "[basx161] Numbers with E",
          "bson": "1800000013640000000000000000000000000000002E3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"00E-9\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0E-9\"}}"
       },
       {
          "description": "[basx649] Zeros",
          "bson": "180000001364000000000000000000000000000000503000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.0E+9\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0E+8\"}}"
       },
       {
          "description": "[basx669] Zeros",
          "bson": "1800000013640000000000000000000000000000002C3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.0E-9\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0E-10\"}}"
       },
       {
          "description": "[basx062] strings without E cannot generate E in result",
          "bson": "18000000136400185C0ACE00000000000000000000383000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"+0345678.5432\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"345678.5432\"}}"
       },
       {
          "description": "[basx001] conform to rules and exponent will be in permitted range).",
          "bson": "180000001364000000000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0\"}}"
       },
       {
          "description": "[basx017] conform to rules and exponent will be in permitted range).",
          "bson": "18000000136400000000000000000000000000000040B000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0\"}}"
       },
       {
          "description": "[basx611] Zeros",
          "bson": "180000001364000000000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0\"}}"
       },
       {
          "description": "[basx613] Zeros",
          "bson": "18000000136400000000000000000000000000000040B000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0.\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"-0\"}}"
       },
       {
          "description": "[basx685] Zeros",
          "bson": "180000001364000000000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0\"}}"
       },
       {
          "description": "[basx688] Zeros",
          "bson": "180000001364000000000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"+0.\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0\"}}"
       },
       {
          "description": "[basx689] Zeros",
          "bson": "18000000136400000000000000000000000000000040B000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0.\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"-0\"}}"
       },
       {
          "description": "[basx650] Zeros",
          "bson": "180000001364000000000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0E+0\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0\"}}"
       },
       {
          "description": "[basx651] Zeros",
          "bson": "180000001364000000000000000000000000000000423000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0E+1\"}}"
       },
       {
          "description": "[basx298] some more negative zeros [systematic tests below]",
          "bson": "1800000013640000000000000000000000000000003CB000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0E-2\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"-0.00\"}}"
       },
       {
          "description": "[basx652] Zeros",
          "bson": "180000001364000000000000000000000000000000443000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0E+2\"}}"
       },
       {
          "description": "[basx299] some more negative zeros [systematic tests below]",
          "bson": "1800000013640000000000000000000000000000003AB000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0E-3\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"-0.000\"}}"
       },
       {
          "description": "[basx653] Zeros",
          "bson": "180000001364000000000000000000000000000000463000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0E+3\"}}"
       },
       {
          "description": "[basx654] Zeros",
          "bson": "180000001364000000000000000000000000000000483000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0E+4\"}}"
       },
       {
          "description": "[basx655] Zeros",
          "bson": "1800000013640000000000000000000000000000004A3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0E+5\"}}"
       },
       {
          "description": "[basx656] Zeros",
          "bson": "1800000013640000000000000000000000000000004C3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0E+6\"}}"
       },
       {
          "description": "[basx657] Zeros",
          "bson": "1800000013640000000000000000000000000000004E3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0E+7\"}}"
       },
       {
          "description": "[basx658] Zeros",
          "bson": "180000001364000000000000000000000000000000503000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0E+8\"}}"
       },
       {
          "description": "[basx138] Numbers with E",
          "bson": "180000001364000000000000000000000000000000523000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"+0E+9\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0E+9\"}}"
       },
       {
          "description": "[basx139] Numbers with E",
          "bson": "18000000136400000000000000000000000000000052B000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0E+9\"}}"
       },
       {
          "description": "[basx144] Numbers with E",
          "bson": "180000001364000000000000000000000000000000523000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0E+9\"}}"
       },
       {
          "description": "[basx154] Numbers with E",
          "bson": "180000001364000000000000000000000000000000523000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0E9\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0E+9\"}}"
       },
       {
          "description": "[basx659] Zeros",
          "bson": "180000001364000000000000000000000000000000523000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0E+9\"}}"
       },
       {
          "description": "[basx042] strings without E cannot generate E in result",
          "bson": "18000000136400FC040000000000000000000000003C3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"+12.76\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"12.76\"}}"
       },
       {
          "description": "[basx143] Numbers with E",
          "bson": "180000001364000100000000000000000000000000523000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"+1E+009\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+9\"}}"
       },
       {
          "description": "[basx061] strings without E cannot generate E in result",
          "bson": "18000000136400185C0ACE00000000000000000000383000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"+345678.5432\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"345678.5432\"}}"
       },
       {
          "description": "[basx036] conform to rules and exponent will be in permitted range).",
          "bson": "1800000013640015CD5B0700000000000000000000203000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.0000000123456789\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.23456789E-8\"}}"
       },
       {
          "description": "[basx035] conform to rules and exponent will be in permitted range).",
          "bson": "1800000013640015CD5B0700000000000000000000223000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.000000123456789\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.23456789E-7\"}}"
       },
       {
          "description": "[basx034] conform to rules and exponent will be in permitted range).",
          "bson": "1800000013640015CD5B0700000000000000000000243000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.00000123456789\"}}"
       },
       {
          "description": "[basx053] strings without E cannot generate E in result",
          "bson": "180000001364003200000000000000000000000000323000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.0000050\"}}"
       },
       {
          "description": "[basx033] conform to rules and exponent will be in permitted range).",
          "bson": "1800000013640015CD5B0700000000000000000000263000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.0000123456789\"}}"
       },
       {
          "description": "[basx016] conform to rules and exponent will be in permitted range).",
          "bson": "180000001364000C000000000000000000000000003A3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.012\"}}"
       },
       {
          "description": "[basx015] conform to rules and exponent will be in permitted range).",
          "bson": "180000001364007B000000000000000000000000003A3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.123\"}}"
       },
       {
          "description": "[basx037] conform to rules and exponent will be in permitted range).",
          "bson": "1800000013640078DF0D8648700000000000000000223000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.123456789012344\"}}"
       },
       {
          "description": "[basx038] conform to rules and exponent will be in permitted range).",
          "bson": "1800000013640079DF0D8648700000000000000000223000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.123456789012345\"}}"
       },
       {
          "description": "[basx250] Numbers with E",
          "bson": "18000000136400F104000000000000000000000000383000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.1265\"}}"
       },
       {
          "description": "[basx257] Numbers with E",
          "bson": "18000000136400F104000000000000000000000000383000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.1265E-0\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.1265\"}}"
       },
       {
          "description": "[basx256] Numbers with E",
          "bson": "18000000136400F104000000000000000000000000363000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.1265E-1\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.01265\"}}"
       },
       {
          "description": "[basx258] Numbers with E",
          "bson": "18000000136400F1040000000000000000000000003A3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.1265E+1\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.265\"}}"
       },
       {
          "description": "[basx251] Numbers with E",
          "bson": "18000000136400F104000000000000000000000000103000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.1265E-20\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.265E-21\"}}"
       },
       {
          "description": "[basx263] Numbers with E",
          "bson": "18000000136400F104000000000000000000000000603000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.1265E+20\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.265E+19\"}}"
       },
       {
          "description": "[basx255] Numbers with E",
          "bson": "18000000136400F104000000000000000000000000343000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.1265E-2\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.001265\"}}"
       },
       {
          "description": "[basx259] Numbers with E",
          "bson": "18000000136400F1040000000000000000000000003C3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.1265E+2\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"12.65\"}}"
       },
       {
          "description": "[basx254] Numbers with E",
          "bson": "18000000136400F104000000000000000000000000323000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.1265E-3\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.0001265\"}}"
       },
       {
          "description": "[basx260] Numbers with E",
          "bson": "18000000136400F1040000000000000000000000003E3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.1265E+3\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"126.5\"}}"
       },
       {
          "description": "[basx253] Numbers with E",
          "bson": "18000000136400F104000000000000000000000000303000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.1265E-4\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.00001265\"}}"
       },
       {
          "description": "[basx261] Numbers with E",
          "bson": "18000000136400F104000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.1265E+4\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1265\"}}"
       },
       {
          "description": "[basx252] Numbers with E",
          "bson": "18000000136400F104000000000000000000000000283000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.1265E-8\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.265E-9\"}}"
       },
       {
          "description": "[basx262] Numbers with E",
          "bson": "18000000136400F104000000000000000000000000483000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.1265E+8\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.265E+7\"}}"
       },
       {
          "description": "[basx159] Numbers with E",
          "bson": "1800000013640049000000000000000000000000002E3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.73e-7\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"7.3E-8\"}}"
       },
       {
          "description": "[basx004] conform to rules and exponent will be in permitted range).",
          "bson": "1800000013640064000000000000000000000000003C3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.00\"}}"
       },
       {
          "description": "[basx003] conform to rules and exponent will be in permitted range).",
          "bson": "180000001364000A000000000000000000000000003E3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0\"}}"
       },
       {
          "description": "[basx002] conform to rules and exponent will be in permitted range).",
          "bson": "180000001364000100000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1\"}}"
       },
       {
          "description": "[basx148] Numbers with E",
          "bson": "180000001364000100000000000000000000000000523000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+009\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+9\"}}"
       },
       {
          "description": "[basx153] Numbers with E",
          "bson": "180000001364000100000000000000000000000000523000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E009\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+9\"}}"
       },
       {
          "description": "[basx141] Numbers with E",
          "bson": "180000001364000100000000000000000000000000523000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1e+09\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+9\"}}"
       },
       {
          "description": "[basx146] Numbers with E",
          "bson": "180000001364000100000000000000000000000000523000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+09\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+9\"}}"
       },
       {
          "description": "[basx151] Numbers with E",
          "bson": "180000001364000100000000000000000000000000523000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1e09\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+9\"}}"
       },
       {
          "description": "[basx142] Numbers with E",
          "bson": "180000001364000100000000000000000000000000F43000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+90\"}}"
       },
       {
          "description": "[basx147] Numbers with E",
          "bson": "180000001364000100000000000000000000000000F43000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1e+90\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+90\"}}"
       },
       {
          "description": "[basx152] Numbers with E",
          "bson": "180000001364000100000000000000000000000000F43000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E90\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+90\"}}"
       },
       {
          "description": "[basx140] Numbers with E",
          "bson": "180000001364000100000000000000000000000000523000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+9\"}}"
       },
       {
          "description": "[basx150] Numbers with E",
          "bson": "180000001364000100000000000000000000000000523000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E9\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+9\"}}"
       },
       {
          "description": "[basx014] conform to rules and exponent will be in permitted range).",
          "bson": "18000000136400D2040000000000000000000000003A3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.234\"}}"
       },
       {
          "description": "[basx170] Numbers with E",
          "bson": "18000000136400F1040000000000000000000000003A3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.265\"}}"
       },
       {
          "description": "[basx177] Numbers with E",
          "bson": "18000000136400F1040000000000000000000000003A3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.265E-0\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.265\"}}"
       },
       {
          "description": "[basx176] Numbers with E",
          "bson": "18000000136400F104000000000000000000000000383000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.265E-1\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.1265\"}}"
       },
       {
          "description": "[basx178] Numbers with E",
          "bson": "18000000136400F1040000000000000000000000003C3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.265E+1\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"12.65\"}}"
       },
       {
          "description": "[basx171] Numbers with E",
          "bson": "18000000136400F104000000000000000000000000123000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.265E-20\"}}"
       },
       {
          "description": "[basx183] Numbers with E",
          "bson": "18000000136400F104000000000000000000000000623000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.265E+20\"}}"
       },
       {
          "description": "[basx175] Numbers with E",
          "bson": "18000000136400F104000000000000000000000000363000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.265E-2\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.01265\"}}"
       },
       {
          "description": "[basx179] Numbers with E",
          "bson": "18000000136400F1040000000000000000000000003E3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.265E+2\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"126.5\"}}"
       },
       {
          "description": "[basx174] Numbers with E",
          "bson": "18000000136400F104000000000000000000000000343000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.265E-3\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.001265\"}}"
       },
       {
          "description": "[basx180] Numbers with E",
          "bson": "18000000136400F104000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.265E+3\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1265\"}}"
       },
       {
          "description": "[basx173] Numbers with E",
          "bson": "18000000136400F104000000000000000000000000323000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.265E-4\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.0001265\"}}"
       },
       {
          "description": "[basx181] Numbers with E",
          "bson": "18000000136400F104000000000000000000000000423000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.265E+4\"}}"
       },
       {
          "description": "[basx172] Numbers with E",
          "bson": "18000000136400F1040000000000000000000000002A3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.265E-8\"}}"
       },
       {
          "description": "[basx182] Numbers with E",
          "bson": "18000000136400F1040000000000000000000000004A3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.265E+8\"}}"
       },
       {
          "description": "[basx157] Numbers with E",
          "bson": "180000001364000400000000000000000000000000523000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"4E+9\"}}"
       },
       {
          "description": "[basx067] examples",
          "bson": "180000001364000500000000000000000000000000343000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"5E-6\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.000005\"}}"
       },
       {
          "description": "[basx069] examples",
          "bson": "180000001364000500000000000000000000000000323000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"5E-7\"}}"
       },
       {
          "description": "[basx385] Engineering notation tests",
          "bson": "180000001364000700000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"7E0\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"7\"}}"
       },
       {
          "description": "[basx365] Engineering notation tests",
          "bson": "180000001364000700000000000000000000000000543000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"7E10\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"7E+10\"}}"
       },
       {
          "description": "[basx405] Engineering notation tests",
          "bson": "1800000013640007000000000000000000000000002C3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"7E-10\"}}"
       },
       {
          "description": "[basx363] Engineering notation tests",
          "bson": "180000001364000700000000000000000000000000563000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"7E11\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"7E+11\"}}"
       },
       {
          "description": "[basx407] Engineering notation tests",
          "bson": "1800000013640007000000000000000000000000002A3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"7E-11\"}}"
       },
       {
          "description": "[basx361] Engineering notation tests",
          "bson": "180000001364000700000000000000000000000000583000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"7E12\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"7E+12\"}}"
       },
       {
          "description": "[basx409] Engineering notation tests",
          "bson": "180000001364000700000000000000000000000000283000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"7E-12\"}}"
       },
       {
          "description": "[basx411] Engineering notation tests",
          "bson": "180000001364000700000000000000000000000000263000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"7E-13\"}}"
       },
       {
          "description": "[basx383] Engineering notation tests",
          "bson": "180000001364000700000000000000000000000000423000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"7E1\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"7E+1\"}}"
       },
       {
          "description": "[basx387] Engineering notation tests",
          "bson": "1800000013640007000000000000000000000000003E3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"7E-1\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.7\"}}"
       },
       {
          "description": "[basx381] Engineering notation tests",
          "bson": "180000001364000700000000000000000000000000443000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"7E2\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"7E+2\"}}"
       },
       {
          "description": "[basx389] Engineering notation tests",
          "bson": "1800000013640007000000000000000000000000003C3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"7E-2\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.07\"}}"
       },
       {
          "description": "[basx379] Engineering notation tests",
          "bson": "180000001364000700000000000000000000000000463000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"7E3\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"7E+3\"}}"
       },
       {
          "description": "[basx391] Engineering notation tests",
          "bson": "1800000013640007000000000000000000000000003A3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"7E-3\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.007\"}}"
       },
       {
          "description": "[basx377] Engineering notation tests",
          "bson": "180000001364000700000000000000000000000000483000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"7E4\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"7E+4\"}}"
       },
       {
          "description": "[basx393] Engineering notation tests",
          "bson": "180000001364000700000000000000000000000000383000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"7E-4\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.0007\"}}"
       },
       {
          "description": "[basx375] Engineering notation tests",
          "bson": "1800000013640007000000000000000000000000004A3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"7E5\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"7E+5\"}}"
       },
       {
          "description": "[basx395] Engineering notation tests",
          "bson": "180000001364000700000000000000000000000000363000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"7E-5\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.00007\"}}"
       },
       {
          "description": "[basx373] Engineering notation tests",
          "bson": "1800000013640007000000000000000000000000004C3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"7E6\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"7E+6\"}}"
       },
       {
          "description": "[basx397] Engineering notation tests",
          "bson": "180000001364000700000000000000000000000000343000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"7E-6\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.000007\"}}"
       },
       {
          "description": "[basx371] Engineering notation tests",
          "bson": "1800000013640007000000000000000000000000004E3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"7E7\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"7E+7\"}}"
       },
       {
          "description": "[basx399] Engineering notation tests",
          "bson": "180000001364000700000000000000000000000000323000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"7E-7\"}}"
       },
       {
          "description": "[basx369] Engineering notation tests",
          "bson": "180000001364000700000000000000000000000000503000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"7E8\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"7E+8\"}}"
       },
       {
          "description": "[basx401] Engineering notation tests",
          "bson": "180000001364000700000000000000000000000000303000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"7E-8\"}}"
       },
       {
          "description": "[basx367] Engineering notation tests",
          "bson": "180000001364000700000000000000000000000000523000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"7E9\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"7E+9\"}}"
       },
       {
          "description": "[basx403] Engineering notation tests",
          "bson": "1800000013640007000000000000000000000000002E3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"7E-9\"}}"
       },
       {
          "description": "[basx007] conform to rules and exponent will be in permitted range).",
          "bson": "1800000013640064000000000000000000000000003E3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"10.0\"}}"
       },
       {
          "description": "[basx005] conform to rules and exponent will be in permitted range).",
          "bson": "180000001364000A00000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"10\"}}"
       },
       {
          "description": "[basx165] Numbers with E",
          "bson": "180000001364000A00000000000000000000000000523000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"10E+009\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0E+10\"}}"
       },
       {
          "description": "[basx163] Numbers with E",
          "bson": "180000001364000A00000000000000000000000000523000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"10E+09\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0E+10\"}}"
       },
       {
          "description": "[basx325] Engineering notation tests",
          "bson": "180000001364000A00000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"10e0\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"10\"}}"
       },
       {
          "description": "[basx305] Engineering notation tests",
          "bson": "180000001364000A00000000000000000000000000543000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"10e10\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0E+11\"}}"
       },
       {
          "description": "[basx345] Engineering notation tests",
          "bson": "180000001364000A000000000000000000000000002C3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"10e-10\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0E-9\"}}"
       },
       {
          "description": "[basx303] Engineering notation tests",
          "bson": "180000001364000A00000000000000000000000000563000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"10e11\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0E+12\"}}"
       },
       {
          "description": "[basx347] Engineering notation tests",
          "bson": "180000001364000A000000000000000000000000002A3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"10e-11\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0E-10\"}}"
       },
       {
          "description": "[basx301] Engineering notation tests",
          "bson": "180000001364000A00000000000000000000000000583000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"10e12\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0E+13\"}}"
       },
       {
          "description": "[basx349] Engineering notation tests",
          "bson": "180000001364000A00000000000000000000000000283000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"10e-12\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0E-11\"}}"
       },
       {
          "description": "[basx351] Engineering notation tests",
          "bson": "180000001364000A00000000000000000000000000263000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"10e-13\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0E-12\"}}"
       },
       {
          "description": "[basx323] Engineering notation tests",
          "bson": "180000001364000A00000000000000000000000000423000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"10e1\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0E+2\"}}"
       },
       {
          "description": "[basx327] Engineering notation tests",
          "bson": "180000001364000A000000000000000000000000003E3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"10e-1\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0\"}}"
       },
       {
          "description": "[basx321] Engineering notation tests",
          "bson": "180000001364000A00000000000000000000000000443000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"10e2\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0E+3\"}}"
       },
       {
          "description": "[basx329] Engineering notation tests",
          "bson": "180000001364000A000000000000000000000000003C3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"10e-2\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.10\"}}"
       },
       {
          "description": "[basx319] Engineering notation tests",
          "bson": "180000001364000A00000000000000000000000000463000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"10e3\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0E+4\"}}"
       },
       {
          "description": "[basx331] Engineering notation tests",
          "bson": "180000001364000A000000000000000000000000003A3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"10e-3\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.010\"}}"
       },
       {
          "description": "[basx317] Engineering notation tests",
          "bson": "180000001364000A00000000000000000000000000483000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"10e4\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0E+5\"}}"
       },
       {
          "description": "[basx333] Engineering notation tests",
          "bson": "180000001364000A00000000000000000000000000383000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"10e-4\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.0010\"}}"
       },
       {
          "description": "[basx315] Engineering notation tests",
          "bson": "180000001364000A000000000000000000000000004A3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"10e5\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0E+6\"}}"
       },
       {
          "description": "[basx335] Engineering notation tests",
          "bson": "180000001364000A00000000000000000000000000363000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"10e-5\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.00010\"}}"
       },
       {
          "description": "[basx313] Engineering notation tests",
          "bson": "180000001364000A000000000000000000000000004C3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"10e6\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0E+7\"}}"
       },
       {
          "description": "[basx337] Engineering notation tests",
          "bson": "180000001364000A00000000000000000000000000343000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"10e-6\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.000010\"}}"
       },
       {
          "description": "[basx311] Engineering notation tests",
          "bson": "180000001364000A000000000000000000000000004E3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"10e7\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0E+8\"}}"
       },
       {
          "description": "[basx339] Engineering notation tests",
          "bson": "180000001364000A00000000000000000000000000323000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"10e-7\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.0000010\"}}"
       },
       {
          "description": "[basx309] Engineering notation tests",
          "bson": "180000001364000A00000000000000000000000000503000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"10e8\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0E+9\"}}"
       },
       {
          "description": "[basx341] Engineering notation tests",
          "bson": "180000001364000A00000000000000000000000000303000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"10e-8\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0E-7\"}}"
       },
       {
          "description": "[basx164] Numbers with E",
          "bson": "180000001364000A00000000000000000000000000F43000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"10e+90\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0E+91\"}}"
       },
       {
          "description": "[basx162] Numbers with E",
          "bson": "180000001364000A00000000000000000000000000523000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"10E+9\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0E+10\"}}"
       },
       {
          "description": "[basx307] Engineering notation tests",
          "bson": "180000001364000A00000000000000000000000000523000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"10e9\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0E+10\"}}"
       },
       {
          "description": "[basx343] Engineering notation tests",
          "bson": "180000001364000A000000000000000000000000002E3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"10e-9\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0E-8\"}}"
       },
       {
          "description": "[basx008] conform to rules and exponent will be in permitted range).",
          "bson": "1800000013640065000000000000000000000000003E3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"10.1\"}}"
       },
       {
          "description": "[basx009] conform to rules and exponent will be in permitted range).",
          "bson": "1800000013640068000000000000000000000000003E3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"10.4\"}}"
       },
       {
          "description": "[basx010] conform to rules and exponent will be in permitted range).",
          "bson": "1800000013640069000000000000000000000000003E3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"10.5\"}}"
       },
       {
          "description": "[basx011] conform to rules and exponent will be in permitted range).",
          "bson": "180000001364006A000000000000000000000000003E3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"10.6\"}}"
       },
       {
          "description": "[basx012] conform to rules and exponent will be in permitted range).",
          "bson": "180000001364006D000000000000000000000000003E3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"10.9\"}}"
       },
       {
          "description": "[basx013] conform to rules and exponent will be in permitted range).",
          "bson": "180000001364006E000000000000000000000000003E3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"11.0\"}}"
       },
       {
          "description": "[basx040] strings without E cannot generate E in result",
          "bson": "180000001364000C00000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"12\"}}"
       },
       {
          "description": "[basx190] Numbers with E",
          "bson": "18000000136400F1040000000000000000000000003C3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"12.65\"}}"
       },
       {
          "description": "[basx197] Numbers with E",
          "bson": "18000000136400F1040000000000000000000000003C3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"12.65E-0\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"12.65\"}}"
       },
       {
          "description": "[basx196] Numbers with E",
          "bson": "18000000136400F1040000000000000000000000003A3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"12.65E-1\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.265\"}}"
       },
       {
          "description": "[basx198] Numbers with E",
          "bson": "18000000136400F1040000000000000000000000003E3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"12.65E+1\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"126.5\"}}"
       },
       {
          "description": "[basx191] Numbers with E",
          "bson": "18000000136400F104000000000000000000000000143000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"12.65E-20\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.265E-19\"}}"
       },
       {
          "description": "[basx203] Numbers with E",
          "bson": "18000000136400F104000000000000000000000000643000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"12.65E+20\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.265E+21\"}}"
       },
       {
          "description": "[basx195] Numbers with E",
          "bson": "18000000136400F104000000000000000000000000383000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"12.65E-2\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.1265\"}}"
       },
       {
          "description": "[basx199] Numbers with E",
          "bson": "18000000136400F104000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"12.65E+2\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1265\"}}"
       },
       {
          "description": "[basx194] Numbers with E",
          "bson": "18000000136400F104000000000000000000000000363000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"12.65E-3\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.01265\"}}"
       },
       {
          "description": "[basx200] Numbers with E",
          "bson": "18000000136400F104000000000000000000000000423000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"12.65E+3\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.265E+4\"}}"
       },
       {
          "description": "[basx193] Numbers with E",
          "bson": "18000000136400F104000000000000000000000000343000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"12.65E-4\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.001265\"}}"
       },
       {
          "description": "[basx201] Numbers with E",
          "bson": "18000000136400F104000000000000000000000000443000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"12.65E+4\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.265E+5\"}}"
       },
       {
          "description": "[basx192] Numbers with E",
          "bson": "18000000136400F1040000000000000000000000002C3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"12.65E-8\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.265E-7\"}}"
       },
       {
          "description": "[basx202] Numbers with E",
          "bson": "18000000136400F1040000000000000000000000004C3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"12.65E+8\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.265E+9\"}}"
       },
       {
          "description": "[basx044] strings without E cannot generate E in result",
          "bson": "18000000136400FC040000000000000000000000003C3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"012.76\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"12.76\"}}"
       },
       {
          "description": "[basx042] strings without E cannot generate E in result",
          "bson": "18000000136400FC040000000000000000000000003C3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"12.76\"}}"
       },
       {
          "description": "[basx046] strings without E cannot generate E in result",
          "bson": "180000001364001100000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"17.\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"17\"}}"
       },
       {
          "description": "[basx049] strings without E cannot generate E in result",
          "bson": "180000001364002C00000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0044\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"44\"}}"
       },
       {
          "description": "[basx048] strings without E cannot generate E in result",
          "bson": "180000001364002C00000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"044\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"44\"}}"
       },
       {
          "description": "[basx158] Numbers with E",
          "bson": "180000001364002C00000000000000000000000000523000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"44E+9\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"4.4E+10\"}}"
       },
       {
          "description": "[basx068] examples",
          "bson": "180000001364003200000000000000000000000000323000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"50E-7\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.0000050\"}}"
       },
       {
          "description": "[basx169] Numbers with E",
          "bson": "180000001364006400000000000000000000000000523000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"100e+009\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.00E+11\"}}"
       },
       {
          "description": "[basx167] Numbers with E",
          "bson": "180000001364006400000000000000000000000000523000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"100e+09\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.00E+11\"}}"
       },
       {
          "description": "[basx168] Numbers with E",
          "bson": "180000001364006400000000000000000000000000F43000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"100E+90\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.00E+92\"}}"
       },
       {
          "description": "[basx166] Numbers with E",
          "bson": "180000001364006400000000000000000000000000523000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"100e+9\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.00E+11\"}}"
       },
       {
          "description": "[basx210] Numbers with E",
          "bson": "18000000136400F1040000000000000000000000003E3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"126.5\"}}"
       },
       {
          "description": "[basx217] Numbers with E",
          "bson": "18000000136400F1040000000000000000000000003E3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"126.5E-0\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"126.5\"}}"
       },
       {
          "description": "[basx216] Numbers with E",
          "bson": "18000000136400F1040000000000000000000000003C3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"126.5E-1\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"12.65\"}}"
       },
       {
          "description": "[basx218] Numbers with E",
          "bson": "18000000136400F104000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"126.5E+1\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1265\"}}"
       },
       {
          "description": "[basx211] Numbers with E",
          "bson": "18000000136400F104000000000000000000000000163000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"126.5E-20\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.265E-18\"}}"
       },
       {
          "description": "[basx223] Numbers with E",
          "bson": "18000000136400F104000000000000000000000000663000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"126.5E+20\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.265E+22\"}}"
       },
       {
          "description": "[basx215] Numbers with E",
          "bson": "18000000136400F1040000000000000000000000003A3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"126.5E-2\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.265\"}}"
       },
       {
          "description": "[basx219] Numbers with E",
          "bson": "18000000136400F104000000000000000000000000423000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"126.5E+2\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.265E+4\"}}"
       },
       {
          "description": "[basx214] Numbers with E",
          "bson": "18000000136400F104000000000000000000000000383000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"126.5E-3\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.1265\"}}"
       },
       {
          "description": "[basx220] Numbers with E",
          "bson": "18000000136400F104000000000000000000000000443000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"126.5E+3\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.265E+5\"}}"
       },
       {
          "description": "[basx213] Numbers with E",
          "bson": "18000000136400F104000000000000000000000000363000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"126.5E-4\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.01265\"}}"
       },
       {
          "description": "[basx221] Numbers with E",
          "bson": "18000000136400F104000000000000000000000000463000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"126.5E+4\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.265E+6\"}}"
       },
       {
          "description": "[basx212] Numbers with E",
          "bson": "18000000136400F1040000000000000000000000002E3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"126.5E-8\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.000001265\"}}"
       },
       {
          "description": "[basx222] Numbers with E",
          "bson": "18000000136400F1040000000000000000000000004E3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"126.5E+8\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.265E+10\"}}"
       },
       {
          "description": "[basx006] conform to rules and exponent will be in permitted range).",
          "bson": "18000000136400E803000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1000\"}}"
       },
       {
          "description": "[basx230] Numbers with E",
          "bson": "18000000136400F104000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1265\"}}"
       },
       {
          "description": "[basx237] Numbers with E",
          "bson": "18000000136400F104000000000000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1265E-0\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1265\"}}"
       },
       {
          "description": "[basx236] Numbers with E",
          "bson": "18000000136400F1040000000000000000000000003E3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1265E-1\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"126.5\"}}"
       },
       {
          "description": "[basx238] Numbers with E",
          "bson": "18000000136400F104000000000000000000000000423000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1265E+1\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.265E+4\"}}"
       },
       {
          "description": "[basx231] Numbers with E",
          "bson": "18000000136400F104000000000000000000000000183000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1265E-20\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.265E-17\"}}"
       },
       {
          "description": "[basx243] Numbers with E",
          "bson": "18000000136400F104000000000000000000000000683000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1265E+20\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.265E+23\"}}"
       },
       {
          "description": "[basx235] Numbers with E",
          "bson": "18000000136400F1040000000000000000000000003C3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1265E-2\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"12.65\"}}"
       },
       {
          "description": "[basx239] Numbers with E",
          "bson": "18000000136400F104000000000000000000000000443000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1265E+2\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.265E+5\"}}"
       },
       {
          "description": "[basx234] Numbers with E",
          "bson": "18000000136400F1040000000000000000000000003A3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1265E-3\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.265\"}}"
       },
       {
          "description": "[basx240] Numbers with E",
          "bson": "18000000136400F104000000000000000000000000463000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1265E+3\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.265E+6\"}}"
       },
       {
          "description": "[basx233] Numbers with E",
          "bson": "18000000136400F104000000000000000000000000383000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1265E-4\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.1265\"}}"
       },
       {
          "description": "[basx241] Numbers with E",
          "bson": "18000000136400F104000000000000000000000000483000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1265E+4\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.265E+7\"}}"
       },
       {
          "description": "[basx232] Numbers with E",
          "bson": "18000000136400F104000000000000000000000000303000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1265E-8\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.00001265\"}}"
       },
       {
          "description": "[basx242] Numbers with E",
          "bson": "18000000136400F104000000000000000000000000503000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1265E+8\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.265E+11\"}}"
       },
       {
          "description": "[basx060] strings without E cannot generate E in result",
          "bson": "18000000136400185C0ACE00000000000000000000383000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"345678.5432\"}}"
       },
       {
          "description": "[basx059] strings without E cannot generate E in result",
          "bson": "18000000136400F198670C08000000000000000000363000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0345678.54321\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"345678.54321\"}}"
       },
       {
          "description": "[basx058] strings without E cannot generate E in result",
          "bson": "180000001364006AF90B7C50000000000000000000343000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"345678.543210\"}}"
       },
       {
          "description": "[basx057] strings without E cannot generate E in result",
          "bson": "180000001364006A19562522020000000000000000343000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"2345678.543210\"}}"
       },
       {
          "description": "[basx056] strings without E cannot generate E in result",
          "bson": "180000001364006AB9C8733A0B0000000000000000343000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"12345678.543210\"}}"
       },
       {
          "description": "[basx031] conform to rules and exponent will be in permitted range).",
          "bson": "1800000013640040AF0D8648700000000000000000343000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"123456789.000000\"}}"
       },
       {
          "description": "[basx030] conform to rules and exponent will be in permitted range).",
          "bson": "1800000013640080910F8648700000000000000000343000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"123456789.123456\"}}"
       },
       {
          "description": "[basx032] conform to rules and exponent will be in permitted range).",
          "bson": "1800000013640080910F8648700000000000000000403000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"123456789123456\"}}"
       }
    ]
}
`},

	{"decimal128-4.json", `
{
    "description": "Decimal128",
    "bson_type": "0x13",
    "test_key": "d",
    "valid": [
       {
          "description": "[basx023] conform to rules and exponent will be in permitted range).",
          "bson": "1800000013640001000000000000000000000000003EB000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0.1\"}}"
       },

       {
          "description": "[basx045] strings without E cannot generate E in result",
          "bson": "1800000013640003000000000000000000000000003A3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"+0.003\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.003\"}}"
       },
       {
          "description": "[basx610] Zeros",
          "bson": "1800000013640000000000000000000000000000003E3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \".0\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.0\"}}"
       },
       {
          "description": "[basx612] Zeros",
          "bson": "1800000013640000000000000000000000000000003EB000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"-.0\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"-0.0\"}}"
       },
       {
          "description": "[basx043] strings without E cannot generate E in result",
          "bson": "18000000136400FC040000000000000000000000003C3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"+12.76\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"12.76\"}}"
       },
       {
          "description": "[basx055] strings without E cannot generate E in result",
          "bson": "180000001364000500000000000000000000000000303000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.00000005\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"5E-8\"}}"
       },
       {
          "description": "[basx054] strings without E cannot generate E in result",
          "bson": "180000001364000500000000000000000000000000323000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.0000005\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"5E-7\"}}"
       },
       {
          "description": "[basx052] strings without E cannot generate E in result",
          "bson": "180000001364000500000000000000000000000000343000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.000005\"}}"
       },
       {
          "description": "[basx051] strings without E cannot generate E in result",
          "bson": "180000001364000500000000000000000000000000363000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"00.00005\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.00005\"}}"
       },
       {
          "description": "[basx050] strings without E cannot generate E in result",
          "bson": "180000001364000500000000000000000000000000383000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.0005\"}}"
       },
       {
          "description": "[basx047] strings without E cannot generate E in result",
          "bson": "1800000013640005000000000000000000000000003E3000",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \".5\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.5\"}}"
       },
       {
          "description": "[dqbsr431] check rounding modes heeded (Rounded)",
          "bson": "1800000013640099761CC7B548F377DC80A131C836FE2F00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.1111111111111111111111111111123450\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.111111111111111111111111111112345\"}}"
       },
       {
          "description": "OK2",
          "bson": "18000000136400000000000A5BC138938D44C64D31FC2F00",
          "extjson": "{\"d\" : {\"$numberDecimal\" : \".100000000000000000000000000000000000000000000000000000000000\"}}",
          "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0.1000000000000000000000000000000000\"}}"
       }
    ],
    "parseErrors": [
       {
          "description": "[basx564] Near-specials (Conversion_syntax)",
          "string": "Infi"
       },
       {
          "description": "[basx565] Near-specials (Conversion_syntax)",
          "string": "Infin"
       },
       {
          "description": "[basx566] Near-specials (Conversion_syntax)",
          "string": "Infini"
       },
       {
          "description": "[basx567] Near-specials (Conversion_syntax)",
          "string": "Infinit"
       },
       {
          "description": "[basx568] Near-specials (Conversion_syntax)",
          "string": "-Infinit"
       },
       {
          "description": "[basx590] some baddies with dots and Es and dots and specials (Conversion_syntax)",
          "string": ".Infinity"
       },
       {
          "description": "[basx562] Near-specials (Conversion_syntax)",
          "string": "NaNq"
       },
       {
          "description": "[basx563] Near-specials (Conversion_syntax)",
          "string": "NaNs"
       },
       {
          "description": "[dqbas939] overflow results at different rounding modes (Overflow & Inexact & Rounded)",
          "string": "-7e10000"
       },
       {
          "description": "[dqbsr534] negatives (Rounded & Inexact)",
          "string": "-1.11111111111111111111111111111234650"
       },
       {
          "description": "[dqbsr535] negatives (Rounded & Inexact)",
          "string": "-1.11111111111111111111111111111234551"
       },
       {
          "description": "[dqbsr533] negatives (Rounded & Inexact)",
          "string": "-1.11111111111111111111111111111234550"
       },
       {
          "description": "[dqbsr532] negatives (Rounded & Inexact)",
          "string": "-1.11111111111111111111111111111234549"
       },
       {
          "description": "[dqbsr432] check rounding modes heeded (Rounded & Inexact)",
          "string": "1.11111111111111111111111111111234549"
       },
       {
          "description": "[dqbsr433] check rounding modes heeded (Rounded & Inexact)",
          "string": "1.11111111111111111111111111111234550"
       },
       {
          "description": "[dqbsr435] check rounding modes heeded (Rounded & Inexact)",
          "string": "1.11111111111111111111111111111234551"
       },
       {
          "description": "[dqbsr434] check rounding modes heeded (Rounded & Inexact)",
          "string": "1.11111111111111111111111111111234650"
       },
       {
          "description": "[dqbas938] overflow results at different rounding modes (Overflow & Inexact & Rounded)",
          "string": "7e10000"
       },
       {
          "description": "Inexact rounding#1",
          "string": "100000000000000000000000000000000000000000000000000000000001"
       },
       {
          "description": "Inexact rounding#2",
          "string": "1E-6177"
       }
    ]
}
`},

	{"decimal128-5.json", `
{
    "description": "Decimal128",
    "bson_type": "0x13",
    "test_key": "d",
    "valid": [
        {
            "description": "[decq035] fold-downs (more below) (Clamped)",
            "bson": "18000000136400000000807F1BCF85B27059C8A43CFE5F00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.23E+6144\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.230000000000000000000000000000000E+6144\"}}"
        },
        {
            "description": "[decq037] fold-downs (more below) (Clamped)",
            "bson": "18000000136400000000000A5BC138938D44C64D31FE5F00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+6144\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.000000000000000000000000000000000E+6144\"}}"
        },
        {
            "description": "[decq077] Nmin and below (Subnormal)",
            "bson": "180000001364000000000081EFAC855B416D2DEE04000000",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.100000000000000000000000000000000E-6143\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.00000000000000000000000000000000E-6144\"}}"
        },
        {
            "description": "[decq078] Nmin and below (Subnormal)",
            "bson": "180000001364000000000081EFAC855B416D2DEE04000000",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.00000000000000000000000000000000E-6144\"}}"
        },
        {
            "description": "[decq079] Nmin and below (Subnormal)",
            "bson": "180000001364000A00000000000000000000000000000000",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.000000000000000000000000000000010E-6143\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0E-6175\"}}"
        },
        {
            "description": "[decq080] Nmin and below (Subnormal)",
            "bson": "180000001364000A00000000000000000000000000000000",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0E-6175\"}}"
        },
        {
            "description": "[decq081] Nmin and below (Subnormal)",
            "bson": "180000001364000100000000000000000000000000020000",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.00000000000000000000000000000001E-6143\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1E-6175\"}}"
        },
        {
            "description": "[decq082] Nmin and below (Subnormal)",
            "bson": "180000001364000100000000000000000000000000020000",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E-6175\"}}"
        },
        {
            "description": "[decq083] Nmin and below (Subnormal)",
            "bson": "180000001364000100000000000000000000000000000000",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"0.000000000000000000000000000000001E-6143\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1E-6176\"}}"
        },
        {
            "description": "[decq084] Nmin and below (Subnormal)",
            "bson": "180000001364000100000000000000000000000000000000",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E-6176\"}}"
        },
        {
            "description": "[decq090] underflows cannot be tested for simple copies, check edge cases (Subnormal)",
            "bson": "180000001364000100000000000000000000000000000000",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1e-6176\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1E-6176\"}}"
        },
        {
            "description": "[decq100] underflows cannot be tested for simple copies, check edge cases (Subnormal)",
            "bson": "18000000136400FFFFFFFF095BC138938D44C64D31000000",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"999999999999999999999999999999999e-6176\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"9.99999999999999999999999999999999E-6144\"}}"
        },
        {
            "description": "[decq130] fold-downs (more below) (Clamped)",
            "bson": "18000000136400000000807F1BCF85B27059C8A43CFEDF00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"-1.23E+6144\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"-1.230000000000000000000000000000000E+6144\"}}"
        },
        {
            "description": "[decq132] fold-downs (more below) (Clamped)",
            "bson": "18000000136400000000000A5BC138938D44C64D31FEDF00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"-1E+6144\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"-1.000000000000000000000000000000000E+6144\"}}"
        },
        {
            "description": "[decq177] Nmin and below (Subnormal)",
            "bson": "180000001364000000000081EFAC855B416D2DEE04008000",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0.100000000000000000000000000000000E-6143\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"-1.00000000000000000000000000000000E-6144\"}}"
        },
        {
            "description": "[decq178] Nmin and below (Subnormal)",
            "bson": "180000001364000000000081EFAC855B416D2DEE04008000",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"-1.00000000000000000000000000000000E-6144\"}}"
        },
        {
            "description": "[decq179] Nmin and below (Subnormal)",
            "bson": "180000001364000A00000000000000000000000000008000",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0.000000000000000000000000000000010E-6143\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"-1.0E-6175\"}}"
        },
        {
            "description": "[decq180] Nmin and below (Subnormal)",
            "bson": "180000001364000A00000000000000000000000000008000",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"-1.0E-6175\"}}"
        },
        {
            "description": "[decq181] Nmin and below (Subnormal)",
            "bson": "180000001364000100000000000000000000000000028000",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0.00000000000000000000000000000001E-6143\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"-1E-6175\"}}"
        },
        {
            "description": "[decq182] Nmin and below (Subnormal)",
            "bson": "180000001364000100000000000000000000000000028000",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"-1E-6175\"}}"
        },
        {
            "description": "[decq183] Nmin and below (Subnormal)",
            "bson": "180000001364000100000000000000000000000000008000",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0.000000000000000000000000000000001E-6143\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"-1E-6176\"}}"
        },
        {
            "description": "[decq184] Nmin and below (Subnormal)",
            "bson": "180000001364000100000000000000000000000000008000",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"-1E-6176\"}}"
        },
        {
            "description": "[decq190] underflow edge cases (Subnormal)",
            "bson": "180000001364000100000000000000000000000000008000",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"-1e-6176\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"-1E-6176\"}}"
        },
        {
            "description": "[decq200] underflow edge cases (Subnormal)",
            "bson": "18000000136400FFFFFFFF095BC138938D44C64D31008000",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"-999999999999999999999999999999999e-6176\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"-9.99999999999999999999999999999999E-6144\"}}"
        },
        {
            "description": "[decq400] zeros (Clamped)",
            "bson": "180000001364000000000000000000000000000000000000",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"0E-8000\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0E-6176\"}}"
        },
        {
            "description": "[decq401] zeros (Clamped)",
            "bson": "180000001364000000000000000000000000000000000000",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"0E-6177\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0E-6176\"}}"
        },
        {
            "description": "[decq414] clamped zeros... (Clamped)",
            "bson": "180000001364000000000000000000000000000000FE5F00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"0E+6112\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0E+6111\"}}"
        },
        {
            "description": "[decq416] clamped zeros... (Clamped)",
            "bson": "180000001364000000000000000000000000000000FE5F00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"0E+6144\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0E+6111\"}}"
        },
        {
            "description": "[decq418] clamped zeros... (Clamped)",
            "bson": "180000001364000000000000000000000000000000FE5F00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"0E+8000\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"0E+6111\"}}"
        },
        {
            "description": "[decq420] negative zeros (Clamped)",
            "bson": "180000001364000000000000000000000000000000008000",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0E-8000\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"-0E-6176\"}}"
        },
        {
            "description": "[decq421] negative zeros (Clamped)",
            "bson": "180000001364000000000000000000000000000000008000",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0E-6177\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"-0E-6176\"}}"
        },
        {
            "description": "[decq434] clamped zeros... (Clamped)",
            "bson": "180000001364000000000000000000000000000000FEDF00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0E+6112\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"-0E+6111\"}}"
        },
        {
            "description": "[decq436] clamped zeros... (Clamped)",
            "bson": "180000001364000000000000000000000000000000FEDF00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0E+6144\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"-0E+6111\"}}"
        },
        {
            "description": "[decq438] clamped zeros... (Clamped)",
            "bson": "180000001364000000000000000000000000000000FEDF00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"-0E+8000\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"-0E+6111\"}}"
        },
        {
            "description": "[decq601] fold-down full sequence (Clamped)",
            "bson": "18000000136400000000000A5BC138938D44C64D31FE5F00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+6144\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.000000000000000000000000000000000E+6144\"}}"
        },
        {
            "description": "[decq603] fold-down full sequence (Clamped)",
            "bson": "180000001364000000000081EFAC855B416D2DEE04FE5F00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+6143\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.00000000000000000000000000000000E+6143\"}}"
        },
        {
            "description": "[decq605] fold-down full sequence (Clamped)",
            "bson": "1800000013640000000080264B91C02220BE377E00FE5F00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+6142\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0000000000000000000000000000000E+6142\"}}"
        },
        {
            "description": "[decq607] fold-down full sequence (Clamped)",
            "bson": "1800000013640000000040EAED7446D09C2C9F0C00FE5F00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+6141\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.000000000000000000000000000000E+6141\"}}"
        },
        {
            "description": "[decq609] fold-down full sequence (Clamped)",
            "bson": "18000000136400000000A0CA17726DAE0F1E430100FE5F00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+6140\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.00000000000000000000000000000E+6140\"}}"
        },
        {
            "description": "[decq611] fold-down full sequence (Clamped)",
            "bson": "18000000136400000000106102253E5ECE4F200000FE5F00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+6139\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0000000000000000000000000000E+6139\"}}"
        },
        {
            "description": "[decq613] fold-down full sequence (Clamped)",
            "bson": "18000000136400000000E83C80D09F3C2E3B030000FE5F00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+6138\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.000000000000000000000000000E+6138\"}}"
        },
        {
            "description": "[decq615] fold-down full sequence (Clamped)",
            "bson": "18000000136400000000E4D20CC8DCD2B752000000FE5F00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+6137\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.00000000000000000000000000E+6137\"}}"
        },
        {
            "description": "[decq617] fold-down full sequence (Clamped)",
            "bson": "180000001364000000004A48011416954508000000FE5F00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+6136\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0000000000000000000000000E+6136\"}}"
        },
        {
            "description": "[decq619] fold-down full sequence (Clamped)",
            "bson": "18000000136400000000A1EDCCCE1BC2D300000000FE5F00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+6135\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.000000000000000000000000E+6135\"}}"
        },
        {
            "description": "[decq621] fold-down full sequence (Clamped)",
            "bson": "18000000136400000080F64AE1C7022D1500000000FE5F00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+6134\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.00000000000000000000000E+6134\"}}"
        },
        {
            "description": "[decq623] fold-down full sequence (Clamped)",
            "bson": "18000000136400000040B2BAC9E0191E0200000000FE5F00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+6133\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0000000000000000000000E+6133\"}}"
        },
        {
            "description": "[decq625] fold-down full sequence (Clamped)",
            "bson": "180000001364000000A0DEC5ADC935360000000000FE5F00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+6132\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.000000000000000000000E+6132\"}}"
        },
        {
            "description": "[decq627] fold-down full sequence (Clamped)",
            "bson": "18000000136400000010632D5EC76B050000000000FE5F00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+6131\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.00000000000000000000E+6131\"}}"
        },
        {
            "description": "[decq629] fold-down full sequence (Clamped)",
            "bson": "180000001364000000E8890423C78A000000000000FE5F00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+6130\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0000000000000000000E+6130\"}}"
        },
        {
            "description": "[decq631] fold-down full sequence (Clamped)",
            "bson": "18000000136400000064A7B3B6E00D000000000000FE5F00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+6129\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.000000000000000000E+6129\"}}"
        },
        {
            "description": "[decq633] fold-down full sequence (Clamped)",
            "bson": "1800000013640000008A5D78456301000000000000FE5F00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+6128\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.00000000000000000E+6128\"}}"
        },
        {
            "description": "[decq635] fold-down full sequence (Clamped)",
            "bson": "180000001364000000C16FF2862300000000000000FE5F00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+6127\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0000000000000000E+6127\"}}"
        },
        {
            "description": "[decq637] fold-down full sequence (Clamped)",
            "bson": "180000001364000080C6A47E8D0300000000000000FE5F00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+6126\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.000000000000000E+6126\"}}"
        },
        {
            "description": "[decq639] fold-down full sequence (Clamped)",
            "bson": "1800000013640000407A10F35A0000000000000000FE5F00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+6125\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.00000000000000E+6125\"}}"
        },
        {
            "description": "[decq641] fold-down full sequence (Clamped)",
            "bson": "1800000013640000A0724E18090000000000000000FE5F00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+6124\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0000000000000E+6124\"}}"
        },
        {
            "description": "[decq643] fold-down full sequence (Clamped)",
            "bson": "180000001364000010A5D4E8000000000000000000FE5F00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+6123\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.000000000000E+6123\"}}"
        },
        {
            "description": "[decq645] fold-down full sequence (Clamped)",
            "bson": "1800000013640000E8764817000000000000000000FE5F00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+6122\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.00000000000E+6122\"}}"
        },
        {
            "description": "[decq647] fold-down full sequence (Clamped)",
            "bson": "1800000013640000E40B5402000000000000000000FE5F00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+6121\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0000000000E+6121\"}}"
        },
        {
            "description": "[decq649] fold-down full sequence (Clamped)",
            "bson": "1800000013640000CA9A3B00000000000000000000FE5F00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+6120\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.000000000E+6120\"}}"
        },
        {
            "description": "[decq651] fold-down full sequence (Clamped)",
            "bson": "1800000013640000E1F50500000000000000000000FE5F00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+6119\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.00000000E+6119\"}}"
        },
        {
            "description": "[decq653] fold-down full sequence (Clamped)",
            "bson": "180000001364008096980000000000000000000000FE5F00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+6118\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0000000E+6118\"}}"
        },
        {
            "description": "[decq655] fold-down full sequence (Clamped)",
            "bson": "1800000013640040420F0000000000000000000000FE5F00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+6117\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.000000E+6117\"}}"
        },
        {
            "description": "[decq657] fold-down full sequence (Clamped)",
            "bson": "18000000136400A086010000000000000000000000FE5F00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+6116\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.00000E+6116\"}}"
        },
        {
            "description": "[decq659] fold-down full sequence (Clamped)",
            "bson": "180000001364001027000000000000000000000000FE5F00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+6115\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0000E+6115\"}}"
        },
        {
            "description": "[decq661] fold-down full sequence (Clamped)",
            "bson": "18000000136400E803000000000000000000000000FE5F00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+6114\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.000E+6114\"}}"
        },
        {
            "description": "[decq663] fold-down full sequence (Clamped)",
            "bson": "180000001364006400000000000000000000000000FE5F00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+6113\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.00E+6113\"}}"
        },
        {
            "description": "[decq665] fold-down full sequence (Clamped)",
            "bson": "180000001364000A00000000000000000000000000FE5F00",
            "extjson": "{\"d\" : {\"$numberDecimal\" : \"1E+6112\"}}",
            "canonical_extjson": "{\"d\" : {\"$numberDecimal\" : \"1.0E+6112\"}}"
        }
    ]
}
`},

	{"decimal128-6.json", `
{
    "description": "Decimal128",
    "bson_type": "0x13",
    "test_key": "d",
    "parseErrors": [
        {
            "description": "Incomplete Exponent",
            "string": "1e"
        },
        {
            "description": "Exponent at the beginning",
            "string": "E01"
        },
        {
            "description": "Just a decimal place",
            "string": "."
        },
        {
            "description": "2 decimal places",
            "string": "..3"
        },
        {
            "description": "2 decimal places",
            "string": ".13.3"
        },
        {
            "description": "2 decimal places",
            "string": "1..3"
        },
        {
            "description": "2 decimal places",
            "string": "1.3.4"
        },
        {
            "description": "2 decimal places",
            "string": "1.34."
        },
        {
            "description": "Decimal with no digits",
            "string": ".e"
        },
        {
            "description": "2 signs",
            "string": "+-32.4"
        },
        {
            "description": "2 signs",
            "string": "-+32.4"
        },
        {
            "description": "2 negative signs",
            "string": "--32.4"
        },
        {
            "description": "2 negative signs",
            "string": "-32.-4"
        },
        {
            "description": "End in negative sign",
            "string": "32.0-"
        },
        {
            "description": "2 negative signs",
            "string": "32.4E--21"
        },
        {
            "description": "2 negative signs",
            "string": "32.4E-2-1"
        },
        {
            "description": "2 signs",
            "string": "32.4E+-21"
        },
        {
            "description": "Empty string",
            "string": ""
        },
        {
            "description": "leading white space positive number",
            "string": " 1"
        },
        {
            "description": "leading white space negative number",
            "string": " -1"
        },
        {
            "description": "trailing white space",
            "string": "1 "
        },
        {
            "description": "Invalid",
            "string": "E"
        },
        {
            "description": "Invalid",
            "string": "invalid"
        },
        {
            "description": "Invalid",
            "string": "i"
        },
        {
            "description": "Invalid",
            "string": "in"
        },
        {
            "description": "Invalid",
            "string": "-in"
        },
        {
            "description": "Invalid",
            "string": "Na"
        },
        {
            "description": "Invalid",
            "string": "-Na"
        },
        {
            "description": "Invalid",
            "string": "1.23abc"
        },
        {
            "description": "Invalid",
            "string": "1.23abcE+02"
        },
        {
            "description": "Invalid",
            "string": "1.23E+0aabs2"
        }
    ]
}
`},

	{"decimal128-7.json", `
{
    "description": "Decimal128",
    "bson_type": "0x13",
    "test_key": "d",
    "parseErrors": [
       {
          "description": "[basx572] Near-specials (Conversion_syntax)",
          "string": "-9Inf"
       },
       {
          "description": "[basx516] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "-1-"
       },
       {
          "description": "[basx533] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "0000.."
       },
       {
          "description": "[basx534] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": ".0000."
       },
       {
          "description": "[basx535] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "00..00"
       },
       {
          "description": "[basx569] Near-specials (Conversion_syntax)",
          "string": "0Inf"
       },
       {
          "description": "[basx571] Near-specials (Conversion_syntax)",
          "string": "-0Inf"
       },
       {
          "description": "[basx575] Near-specials (Conversion_syntax)",
          "string": "0sNaN"
       },
       {
          "description": "[basx503] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "++1"
       },
       {
          "description": "[basx504] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "--1"
       },
       {
          "description": "[basx505] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "-+1"
       },
       {
          "description": "[basx506] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "+-1"
       },
       {
          "description": "[basx510] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": " +1"
       },
       {
          "description": "[basx513] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": " + 1"
       },
       {
          "description": "[basx514] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": " - 1"
       },
       {
          "description": "[basx501] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "."
       },
       {
          "description": "[basx502] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": ".."
       },
       {
          "description": "[basx519] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": ""
       },
       {
          "description": "[basx525] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "e100"
       },
       {
          "description": "[basx549] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "e+1"
       },
       {
          "description": "[basx577] some baddies with dots and Es and dots and specials (Conversion_syntax)",
          "string": ".e+1"
       },
       {
          "description": "[basx578] some baddies with dots and Es and dots and specials (Conversion_syntax)",
          "string": "+.e+1"
       },
       {
          "description": "[basx581] some baddies with dots and Es and dots and specials (Conversion_syntax)",
          "string": "E+1"
       },
       {
          "description": "[basx582] some baddies with dots and Es and dots and specials (Conversion_syntax)",
          "string": ".E+1"
       },
       {
          "description": "[basx583] some baddies with dots and Es and dots and specials (Conversion_syntax)",
          "string": "+.E+1"
       },
       {
          "description": "[basx579] some baddies with dots and Es and dots and specials (Conversion_syntax)",
          "string": "-.e+"
       },
       {
          "description": "[basx580] some baddies with dots and Es and dots and specials (Conversion_syntax)",
          "string": "-.e"
       },
       {
          "description": "[basx584] some baddies with dots and Es and dots and specials (Conversion_syntax)",
          "string": "-.E+"
       },
       {
          "description": "[basx585] some baddies with dots and Es and dots and specials (Conversion_syntax)",
          "string": "-.E"
       },
       {
          "description": "[basx589] some baddies with dots and Es and dots and specials (Conversion_syntax)",
          "string": "+.Inf"
       },
       {
          "description": "[basx586] some baddies with dots and Es and dots and specials (Conversion_syntax)",
          "string": ".NaN"
       },
       {
          "description": "[basx587] some baddies with dots and Es and dots and specials (Conversion_syntax)",
          "string": "-.NaN"
       },
       {
          "description": "[basx545] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "ONE"
       },
       {
          "description": "[basx561] Near-specials (Conversion_syntax)",
          "string": "qNaN"
       },
       {
          "description": "[basx573] Near-specials (Conversion_syntax)",
          "string": "-sNa"
       },
       {
          "description": "[basx588] some baddies with dots and Es and dots and specials (Conversion_syntax)",
          "string": "+.sNaN"
       },
       {
          "description": "[basx544] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "ten"
       },
       {
          "description": "[basx527] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "u0b65"
       },
       {
          "description": "[basx526] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "u0e5a"
       },
       {
          "description": "[basx515] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "x"
       },
       {
          "description": "[basx574] Near-specials (Conversion_syntax)",
          "string": "xNaN"
       },
       {
          "description": "[basx530] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": ".123.5"
       },
       {
          "description": "[basx500] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "1..2"
       },
       {
          "description": "[basx542] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "1e1.0"
       },
       {
          "description": "[basx553] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "1E+1.2.3"
       },
       {
          "description": "[basx543] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "1e123e"
       },
       {
          "description": "[basx552] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "1E+1.2"
       },
       {
          "description": "[basx546] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "1e.1"
       },
       {
          "description": "[basx547] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "1e1."
       },
       {
          "description": "[basx554] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "1E++1"
       },
       {
          "description": "[basx555] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "1E--1"
       },
       {
          "description": "[basx556] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "1E+-1"
       },
       {
          "description": "[basx557] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "1E-+1"
       },
       {
          "description": "[basx558] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "1E'1"
       },
       {
          "description": "[basx559] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "1E\"1"
       },
       {
          "description": "[basx520] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "1e-"
       },
       {
          "description": "[basx560] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "1E"
       },
       {
          "description": "[basx548] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "1ee"
       },
       {
          "description": "[basx551] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "1.2.1"
       },
       {
          "description": "[basx550] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "1.23.4"
       },
       {
          "description": "[basx529] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "1.34.5"
       },
       {
          "description": "[basx531] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "01.35."
       },
       {
          "description": "[basx532] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "01.35-"
       },
       {
          "description": "[basx518] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "3+"
       },
       {
          "description": "[basx521] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "7e99999a"
       },
       {
          "description": "[basx570] Near-specials (Conversion_syntax)",
          "string": "9Inf"
       },
       {
          "description": "[basx512] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "12 "
       },
       {
          "description": "[basx517] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "12-"
       },
       {
          "description": "[basx507] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "12e"
       },
       {
          "description": "[basx508] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "12e++"
       },
       {
          "description": "[basx509] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "12f4"
       },
       {
          "description": "[basx536] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "111e*123"
       },
       {
          "description": "[basx537] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "111e123-"
       },
       {
          "description": "[basx540] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "111e1*23"
       },
       {
          "description": "[basx538] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "111e+12+"
       },
       {
          "description": "[basx539] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "111e1-3-"
       },
       {
          "description": "[basx541] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "111E1e+3"
       },
       {
          "description": "[basx528] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "123,65"
       },
       {
          "description": "[basx523] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "7e12356789012x"
       },
       {
          "description": "[basx522] The 'baddies' tests from DiagBigDecimal, plus some new ones (Conversion_syntax)",
          "string": "7e123567890x"
       }
    ]
}
`},
}
