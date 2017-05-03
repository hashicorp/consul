// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json5

// JSON value parser state machine.
// Just about at the limit of what is reasonable to write by hand.
// Some parts are a bit tedious, but overall it nicely factors out the
// otherwise common code from the multiple scanning functions
// in this package (Compact, Indent, checkValid, nextValue, etc).
//
// This file starts with two simple examples using the scanner
// before diving into the scanner itself.

import "strconv"

// checkValid verifies that data is valid JSON-encoded data.
// scan is passed in for use by checkValid to avoid an allocation.
func checkValid(data []byte, scan *scanner) error {
	scan.reset()
	for _, c := range data {
		scan.bytes++
		if scan.step(scan, c) == scanError {
			return scan.err
		}
	}
	if scan.eof() == scanError {
		return scan.err
	}
	return nil
}

// nextValue splits data after the next whole JSON value,
// returning that value and the bytes that follow it as separate slices.
// scan is passed in for use by nextValue to avoid an allocation.
func nextValue(data []byte, scan *scanner) (value, rest []byte, err error) {
	scan.reset()
	for i, c := range data {
		v := scan.step(scan, c)
		if v >= scanEndObject {
			switch v {
			// probe the scanner with a space to determine whether we will
			// get scanEnd on the next character. Otherwise, if the next character
			// is not a space, scanEndTop allocates a needless error.
			case scanEndObject, scanEndArray:
				if scan.step(scan, ' ') == scanEnd {
					return data[:i+1], data[i+1:], nil
				}
			case scanError:
				return nil, nil, scan.err
			case scanEnd:
				return data[:i], data[i:], nil
			}
		}
	}
	if scan.eof() == scanError {
		return nil, nil, scan.err
	}
	return data, nil, nil
}

// A SyntaxError is a description of a JSON syntax error.
type SyntaxError struct {
	msg    string // description of error
	Offset int64  // error occurred after reading Offset bytes
}

func (e *SyntaxError) Error() string { return e.msg }

// A scanner is a JSON scanning state machine.
// Callers call scan.reset() and then pass bytes in one at a time
// by calling scan.step(&scan, c) for each byte.
// The return value, referred to as an opcode, tells the
// caller about significant parsing events like beginning
// and ending literals, objects, and arrays, so that the
// caller can follow along if it wishes.
// The return value scanEnd indicates that a single top-level
// JSON value has been completed, *before* the byte that
// just got passed in.  (The indication must be delayed in order
// to recognize the end of numbers: is 123 a whole value or
// the beginning of 12345e+6?).
type scanner struct {
	// The step is a func to be called to execute the next transition.
	// Also tried using an integer constant and a single func
	// with a switch, but using the func directly was 10% faster
	// on a 64-bit Mac Mini, and it's nicer to read.
	step func(*scanner, byte) int

	// Comments are hidden from callers of the scanner, commentEndStep is used
	// to resume normal parsing when a comment ends.
	commentEndStep func(*scanner, byte) int

	// Reached end of top-level value.
	endTop bool

	// Stack of what we're in the middle of - array values, object keys, object values.
	parseState []int

	// Error that happened, if any.
	err error

	// 1-byte redo (see undo method)
	redo      bool
	redoCode  int
	redoState func(*scanner, byte) int

	// total bytes consumed, updated by decoder.Decode
	bytes int64
}

// These values are returned by the state transition functions
// assigned to scanner.state and the method scanner.eof.
// They give details about the current state of the scan that
// callers might be interested to know about.
// It is okay to ignore the return value of any particular
// call to scanner.state: if one call returns scanError,
// every subsequent call will return scanError too.
const (
	// Continue.
	scanContinue     = iota // uninteresting byte
	scanBeginLiteral        // end implied by next result != scanContinue
	scanBeginObject         // begin object
	scanObjectKey           // just finished object key (string)
	scanObjectValue         // just finished non-last object value
	scanEndObject           // end object (implies scanObjectValue if possible)
	scanBeginArray          // begin array
	scanArrayValue          // just finished array value
	scanEndArray            // end array (implies scanArrayValue if possible)
	scanSkipSpace           // space byte; can skip; known to be last "continue" result

	// Stop.
	scanEnd   // top-level value ended *before* this byte; known to be first "stop" result
	scanError // hit an error, scanner.err.
)

// These values are stored in the parseState stack.
// They give the current state of a composite value
// being scanned. If the parser is inside a nested value
// the parseState describes the nested state, outermost at entry 0.
const (
	parseObjectKey   = iota // parsing object key (before colon)
	parseObjectValue        // parsing object value (after colon)
	parseArrayValue         // parsing array value
)

// reset prepares the scanner for use.
// It must be called before calling s.step.
func (s *scanner) reset() {
	s.step = stateBeginValue
	s.parseState = s.parseState[0:0]
	s.err = nil
	s.redo = false
	s.endTop = false
}

// eof tells the scanner that the end of input has been reached.
// It returns a scan status just as s.step does.
func (s *scanner) eof() int {
	if s.err != nil {
		return scanError
	}
	if s.endTop {
		return scanEnd
	}
	s.step(s, ' ')
	if s.endTop {
		return scanEnd
	}
	if s.err == nil {
		s.err = &SyntaxError{"unexpected end of JSON input", s.bytes}
	}
	return scanError
}

// pushParseState pushes a new parse state p onto the parse stack.
func (s *scanner) pushParseState(p int) {
	s.parseState = append(s.parseState, p)
}

// popParseState pops a parse state (already obtained) off the stack
// and updates s.step accordingly.
func (s *scanner) popParseState() {
	n := len(s.parseState) - 1
	s.parseState = s.parseState[0:n]
	s.redo = false
	if n == 0 {
		s.step = stateEndTop
		s.endTop = true
	} else {
		s.step = stateEndValue
	}
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '\f'
}

// stateBeginValueOrEmpty is the state after reading `[`.
func stateBeginValueOrEmpty(s *scanner, c byte) int {
	if c <= ' ' && isSpace(c) {
		return scanSkipSpace
	}
	if c == ']' {
		return stateEndValue(s, c)
	}
	if c == '/' {
		s.step = stateBeginComment
		s.commentEndStep = stateBeginValueOrEmpty
		return scanSkipSpace
	}
	return stateBeginValue(s, c)
}

// stateBeginValue is the state at the beginning of the input.
func stateBeginValue(s *scanner, c byte) int {
	if c <= ' ' && isSpace(c) {
		return scanSkipSpace
	}
	switch c {
	case '{':
		s.step = stateBeginObjectKeyOrEmpty
		s.pushParseState(parseObjectKey)
		return scanBeginObject
	case '[':
		s.step = stateBeginValueOrEmpty
		s.pushParseState(parseArrayValue)
		return scanBeginArray
	case '"':
		s.step = stateInStringDouble
		return scanBeginLiteral
	case '\'':
		s.step = stateInStringSingle
		return scanBeginLiteral
	case '-', '+':
		s.step = stateSign
		return scanBeginLiteral
	case '.':
		s.step = stateDot
		return scanBeginLiteral
	case '0': // beginning of 0.123
		s.step = stateFirst0
		return scanBeginLiteral
	case 't': // beginning of true
		s.step = stateT
		return scanBeginLiteral
	case 'f': // beginning of false
		s.step = stateF
		return scanBeginLiteral
	case 'n': // beginning of null
		s.step = stateN
		return scanBeginLiteral
	case 'I': // beginning of Infinity
		s.step = stateInfinity
		return scanBeginLiteral
	case 'N': // beginning of NaN
		s.step = stateNaN
		return scanBeginLiteral
	case '/':
		s.step = stateBeginComment
		s.commentEndStep = stateBeginValue
		return scanSkipSpace
	}
	if '1' <= c && c <= '9' { // beginning of 1234.5
		s.step = state1
		return scanBeginLiteral
	}
	return s.error(c, "looking for beginning of value")
}

func stateBeginComment(s *scanner, c byte) int {
	if c == '/' {
		s.step = stateInLineComment
		return scanSkipSpace
	}
	if c == '*' {
		s.step = stateInBlockComment
		return scanSkipSpace
	}
	return s.error(c, "potentially starting comment")
}

func stateInLineComment(s *scanner, c byte) int {
	if c == '\r' {
		s.step = stateInLineCommentCR
	}
	if c == '\n' {
		s.step = s.commentEndStep
		s.commentEndStep = nil
	}
	return scanSkipSpace
}

func stateInLineCommentCR(s *scanner, c byte) int {
	if c == '\n' {
		s.step = s.commentEndStep
		s.commentEndStep = nil
		return scanSkipSpace
	}
	return s.commentEndStep(s, c)
}

func stateInBlockComment(s *scanner, c byte) int {
	if c == '*' {
		s.step = stateMaybeEndBlockComment
	}
	return scanSkipSpace
}

func stateMaybeEndBlockComment(s *scanner, c byte) int {
	if c == '/' {
		s.step = s.commentEndStep
		s.commentEndStep = nil
	}
	return scanSkipSpace
}

// stateBeginObjectKeyOrEmpty is the state after reading `{`.
func stateBeginObjectKeyOrEmpty(s *scanner, c byte) int {
	if c <= ' ' && isSpace(c) {
		return scanSkipSpace
	}
	if c == '}' {
		n := len(s.parseState)
		s.parseState[n-1] = parseObjectValue
		return stateEndValue(s, c)
	}
	if c == '/' {
		s.step = stateBeginComment
		s.commentEndStep = stateBeginObjectKeyOrEmpty
		return scanSkipSpace
	}
	return stateBeginObjectKey(s, c)
}

// stateBeginObjectKey is the state after reading `{"key": value,`.
func stateBeginObjectKey(s *scanner, c byte) int {
	if c <= ' ' && isSpace(c) {
		return scanSkipSpace
	}
	if c == '"' {
		s.step = stateInStringDouble
		return scanBeginLiteral
	}
	if c == '\'' {
		s.step = stateInStringSingle
		return scanBeginLiteral
	}
	if isValidKeyLiteralFirstByte(c) {
		s.step = stateInKeyLiteral
		return scanBeginLiteral
	}
	if c == '/' {
		s.step = stateBeginComment
		s.commentEndStep = stateBeginObjectKey
		return scanSkipSpace
	}

	return s.error(c, "looking for beginning of object key")
}

// stateInKeyLiteral is the state when starting to read an object key with no quotes
func stateInKeyLiteral(s *scanner, c byte) int {
	if c == ':' || isSpace(c) {
		return stateEndValue(s, c)
	}
	if !isValidKeyLiteralByte(c) {
		return s.error(c, "in key literal")
	}
	return scanContinue
}

func isValidKeyLiteralFirstByte(c byte) bool {
	return isValidKeyLiteralByte(c) && (c < '0' || c > '9')
}

func isValidKeyLiteralByte(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '$'
}

// stateEndValue is the state after completing a value,
// such as after reading `{}` or `true` or `["x"`.
func stateEndValue(s *scanner, c byte) int {
	n := len(s.parseState)
	if n == 0 {
		// Completed top-level before the current byte.
		s.step = stateEndTop
		s.endTop = true
		return stateEndTop(s, c)
	}
	if c <= ' ' && isSpace(c) {
		s.step = stateEndValue
		return scanSkipSpace
	}
	if c == '/' {
		s.step = stateBeginComment
		s.commentEndStep = stateEndValue
		return scanSkipSpace
	}
	ps := s.parseState[n-1]
	switch ps {
	case parseObjectKey:
		if c == ':' {
			s.parseState[n-1] = parseObjectValue
			s.step = stateBeginValue
			return scanObjectKey
		}
		return s.error(c, "after object key")
	case parseObjectValue:
		if c == ',' {
			s.parseState[n-1] = parseObjectKey
			s.step = stateBeginObjectKeyOrEmpty
			return scanObjectValue
		}
		if c == '}' {
			s.popParseState()
			return scanEndObject
		}
		return s.error(c, "after object key:value pair")
	case parseArrayValue:
		if c == ',' {
			s.step = stateBeginValueOrEmpty
			return scanArrayValue
		}
		if c == ']' {
			s.popParseState()
			return scanEndArray
		}
		return s.error(c, "after array element")
	}
	return s.error(c, "")
}

// stateEndTop is the state after finishing the top-level value,
// such as after reading `{}` or `[1,2,3]`.
// Only space characters should be seen now.
func stateEndTop(s *scanner, c byte) int {
	if c == '/' {
		s.step = stateBeginComment
		s.commentEndStep = stateEndTop
		return scanSkipSpace
	}
	if c != ' ' && c != '\t' && c != '\r' && c != '\n' {
		// Complain about non-space byte on next call.
		s.error(c, "after top-level value")
	}
	return scanEnd
}

// stateInStringDouble is the state after reading `"`.
func stateInStringDouble(s *scanner, c byte) int {
	if c == '"' {
		s.step = stateEndValue
		return scanContinue
	}
	if c == '\\' {
		s.step = stateInStringEsc(stateInStringDouble)
		return scanContinue
	}
	if c < 0x20 {
		return s.error(c, "in string literal")
	}
	return scanContinue
}

// stateInStringSingle is the state after reading `"`.
func stateInStringSingle(s *scanner, c byte) int {
	if c == '\'' {
		s.step = stateEndValue
		return scanContinue
	}
	if c == '\\' {
		s.step = stateInStringEsc(stateInStringSingle)
		return scanContinue
	}
	if c < 0x20 {
		return s.error(c, "in string literal")
	}
	return scanContinue
}

// stateInStringEsc is the state after reading `"\` during a quoted string.
func stateInStringEsc(resume func(s *scanner, c byte) int) func(s *scanner, c byte) int {
	return func(s *scanner, c byte) int {
		switch c {
		case 'b', 'f', 'n', 'r', 't', '\\', '/', '"', '\'', '\n':
			s.step = resume
			return scanContinue
		case 'u':
			s.step = stateInStringEscU(resume)
			return scanContinue
		case '\r':
			s.step = stateInStringEscCR(resume)
			return scanContinue
		}
		return s.error(c, "in string escape code")
	}
}

// stateInStringEscCR is the state after reading `"\\r` during a quoted string.
func stateInStringEscCR(resume func(s *scanner, c byte) int) func(s *scanner, c byte) int {
	return func(s *scanner, c byte) int {
		s.step = resume
		if c == '\n' {
			return scanContinue
		}
		return resume(s, c)
	}
}

// stateInStringEscU is the state after reading `"\u` during a quoted string.
func stateInStringEscU(resume func(s *scanner, c byte) int) func(s *scanner, c byte) int {
	return func(s *scanner, c byte) int {
		if '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F' {
			s.step = stateInStringEscU1(resume)
			return scanContinue
		}
		// numbers
		return s.error(c, "in \\u hexadecimal character escape")
	}
}

// stateInStringEscU1 is the state after reading `"\u1` during a quoted string.
func stateInStringEscU1(resume func(s *scanner, c byte) int) func(s *scanner, c byte) int {
	return func(s *scanner, c byte) int {
		if '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F' {
			s.step = stateInStringEscU12(resume)
			return scanContinue
		}
		// numbers
		return s.error(c, "in \\u hexadecimal character escape")
	}
}

// stateInStringEscU12 is the state after reading `"\u12` during a quoted string.
func stateInStringEscU12(resume func(s *scanner, c byte) int) func(s *scanner, c byte) int {
	return func(s *scanner, c byte) int {
		if '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F' {
			s.step = stateInStringEscU123(resume)
			return scanContinue
		}
		// numbers
		return s.error(c, "in \\u hexadecimal character escape")
	}
}

// stateInStringEscU123 is the state after reading `"\u123` during a quoted string.
func stateInStringEscU123(resume func(s *scanner, c byte) int) func(s *scanner, c byte) int {
	return func(s *scanner, c byte) int {
		if '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F' {
			s.step = resume
			return scanContinue
		}
		// numbers
		return s.error(c, "in \\u hexadecimal character escape")
	}
}

// stateSign is the state after reading `+` or `-` during a number.
func stateSign(s *scanner, c byte) int {
	switch {
	case c == '0':
		s.step = stateFirst0
		return scanContinue
	case '1' <= c && c <= '9':
		s.step = state1
		return scanContinue
	case c == '.':
		s.step = stateSign
		return scanContinue
	case c == 'I':
		s.step = stateInfinity
		return scanContinue
	default:
		return s.error(c, "in numeric literal")
	}
}

// state1 is the state after reading a non-zero integer during a number,
// such as after reading `1` or `100` but not `0`.
func state1(s *scanner, c byte) int {
	if '0' <= c && c <= '9' {
		s.step = state1
		return scanContinue
	}
	return state0(s, c)
}

// stateFirst0 is the state after the first integer in a number is `0`
func stateFirst0(s *scanner, c byte) int {
	switch c {
	case '.':
		s.step = stateDot
		return scanContinue
	case 'e', 'E':
		s.step = stateE
		return scanContinue
	case 'x', 'X':
		s.step = stateFirstHex
		return scanContinue
	default:
		return stateEndValue(s, c)
	}
}

// stateFirstHex is the state after reading 0x in a number
func stateFirstHex(s *scanner, c byte) int {
	if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') {
		s.step = stateHex
		return scanContinue
	}
	return s.error(c, "in hex number")
}

// stateHex is the state after reading the first hex digit in a number
func stateHex(s *scanner, c byte) int {
	if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') {
		return scanContinue
	}
	return stateEndValue(s, c)
}

// state0 is the state after reading `0` during a number.
func state0(s *scanner, c byte) int {
	if c == '.' {
		s.step = stateDot
		return scanContinue
	}
	if c == 'e' || c == 'E' {
		s.step = stateE
		return scanContinue
	}
	return stateEndValue(s, c)
}

// stateDot is the state after reading the integer and decimal point in a number,
// such as after reading `1.`.
func stateDot(s *scanner, c byte) int {
	if '0' <= c && c <= '9' {
		s.step = stateDot0
		return scanContinue
	}
	if c == 'e' || c == 'E' {
		s.step = stateE
		return scanContinue
	}
	return stateEndValue(s, c)
}

// stateDot0 is the state after reading the integer, decimal point, and subsequent
// digits of a number, such as after reading `3.14`.
func stateDot0(s *scanner, c byte) int {
	if '0' <= c && c <= '9' {
		return scanContinue
	}
	if c == 'e' || c == 'E' {
		s.step = stateE
		return scanContinue
	}
	return stateEndValue(s, c)
}

// stateE is the state after reading the mantissa and e in a number,
// such as after reading `314e` or `0.314e`.
func stateE(s *scanner, c byte) int {
	if c == '+' || c == '-' {
		s.step = stateESign
		return scanContinue
	}
	return stateESign(s, c)
}

// stateESign is the state after reading the mantissa, e, and sign in a number,
// such as after reading `314e-` or `0.314e+`.
func stateESign(s *scanner, c byte) int {
	if '0' <= c && c <= '9' {
		s.step = stateE0
		return scanContinue
	}
	return s.error(c, "in exponent of numeric literal")
}

// stateE0 is the state after reading the mantissa, e, optional sign,
// and at least one digit of the exponent in a number,
// such as after reading `314e-2` or `0.314e+1` or `3.14e0`.
func stateE0(s *scanner, c byte) int {
	if '0' <= c && c <= '9' {
		return scanContinue
	}
	return stateEndValue(s, c)
}

// stateT is the state after reading `t`.
func stateT(s *scanner, c byte) int {
	if c == 'r' {
		s.step = stateTr
		return scanContinue
	}
	return s.error(c, "in literal true (expecting 'r')")
}

// stateTr is the state after reading `tr`.
func stateTr(s *scanner, c byte) int {
	if c == 'u' {
		s.step = stateTru
		return scanContinue
	}
	return s.error(c, "in literal true (expecting 'u')")
}

// stateTru is the state after reading `tru`.
func stateTru(s *scanner, c byte) int {
	if c == 'e' {
		s.step = stateEndValue
		return scanContinue
	}
	return s.error(c, "in literal true (expecting 'e')")
}

// stateF is the state after reading `f`.
func stateF(s *scanner, c byte) int {
	if c == 'a' {
		s.step = stateFa
		return scanContinue
	}
	return s.error(c, "in literal false (expecting 'a')")
}

// stateFa is the state after reading `fa`.
func stateFa(s *scanner, c byte) int {
	if c == 'l' {
		s.step = stateFal
		return scanContinue
	}
	return s.error(c, "in literal false (expecting 'l')")
}

// stateFal is the state after reading `fal`.
func stateFal(s *scanner, c byte) int {
	if c == 's' {
		s.step = stateFals
		return scanContinue
	}
	return s.error(c, "in literal false (expecting 's')")
}

// stateFals is the state after reading `fals`.
func stateFals(s *scanner, c byte) int {
	if c == 'e' {
		s.step = stateEndValue
		return scanContinue
	}
	return s.error(c, "in literal false (expecting 'e')")
}

// stateN is the state after reading `n`.
func stateN(s *scanner, c byte) int {
	if c == 'u' {
		s.step = stateNu
		return scanContinue
	}
	return s.error(c, "in literal null (expecting 'u')")
}

// stateNu is the state after reading `nu`.
func stateNu(s *scanner, c byte) int {
	if c == 'l' {
		s.step = stateNul
		return scanContinue
	}
	return s.error(c, "in literal null (expecting 'l')")
}

// stateNul is the state after reading `nul`.
func stateNul(s *scanner, c byte) int {
	if c == 'l' {
		s.step = stateEndValue
		return scanContinue
	}
	return s.error(c, "in literal null (expecting 'l')")
}

func stateInfinity(s *scanner, c byte) int {
	str := "nfinity"
	nextState := func(s *scanner, c byte) int {
		if c == str[0] {
			str = str[1:]
			if str == "" {
				s.step = stateEndValue
			}
			return scanContinue
		}
		return s.error(c, "in literal Infinity (expecting "+quoteChar(str[0])+")")
	}
	s.step = nextState
	return nextState(s, c)
}

func stateNaN(s *scanner, c byte) int {
	str := "aN"
	nextState := func(s *scanner, c byte) int {
		if c == str[0] {
			str = str[1:]
			if str == "" {
				s.step = stateEndValue
			}
			return scanContinue
		}
		return s.error(c, "in literal NaN (expecting "+quoteChar(str[0])+")")
	}
	s.step = nextState
	return nextState(s, c)
}

// stateError is the state after reaching a syntax error,
// such as after reading `[1}` or `5.1.2`.
func stateError(s *scanner, c byte) int {
	return scanError
}

// error records an error and switches to the error state.
func (s *scanner) error(c byte, context string) int {
	s.step = stateError
	s.err = &SyntaxError{"invalid character " + quoteChar(c) + " " + context, s.bytes}
	return scanError
}

// quoteChar formats c as a quoted character literal
func quoteChar(c byte) string {
	// special cases - different from quoted strings
	if c == '\'' {
		return `'\''`
	}
	if c == '"' {
		return `'"'`
	}

	// use quoted string with different quotation marks
	s := strconv.Quote(string(c))
	return "'" + s[1:len(s)-1] + "'"
}

// undo causes the scanner to return scanCode from the next state transition.
// This gives callers a simple 1-byte undo mechanism.
func (s *scanner) undo(scanCode int) {
	if s.redo {
		panic("json: invalid use of scanner")
	}
	s.redoCode = scanCode
	s.redoState = s.step
	s.step = stateRedo
	s.redo = true
}

// stateRedo helps implement the scanner's 1-byte undo.
func stateRedo(s *scanner, c byte) int {
	s.redo = false
	s.step = s.redoState
	return s.redoCode
}
