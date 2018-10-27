package errors

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	golog "log"
	"math/rand"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
)

func TestErrors(t *testing.T) {
	buf := bytes.Buffer{}
	golog.SetOutput(&buf)
	em := errorHandler{eLogger: errorLogger}

	testErr := errors.New("test error")
	tests := []struct {
		next         plugin.Handler
		expectedCode int
		expectedLog  string
		expectedErr  error
	}{
		{
			next:         genErrorHandler(dns.RcodeSuccess, nil),
			expectedCode: dns.RcodeSuccess,
			expectedLog:  "",
			expectedErr:  nil,
		},
		{
			next:         genErrorHandler(dns.RcodeNotAuth, testErr),
			expectedCode: dns.RcodeNotAuth,
			expectedLog:  fmt.Sprintf("%d %s: %v\n", dns.RcodeNotAuth, "example.org. A", testErr),
			expectedErr:  testErr,
		},
	}

	ctx := context.TODO()
	req := new(dns.Msg)
	req.SetQuestion("example.org.", dns.TypeA)

	for i, tc := range tests {
		em.Next = tc.next
		buf.Reset()
		rec := dnstest.NewRecorder(&test.ResponseWriter{})
		code, err := em.ServeDNS(ctx, rec, req)

		if err != tc.expectedErr {
			t.Errorf("Test %d: Expected error %v, but got %v",
				i, tc.expectedErr, err)
		}
		if code != tc.expectedCode {
			t.Errorf("Test %d: Expected status code %d, but got %d",
				i, tc.expectedCode, code)
		}
		if log := buf.String(); !strings.Contains(log, tc.expectedLog) {
			t.Errorf("Test %d: Expected log %q, but got %q",
				i, tc.expectedLog, log)
		}
	}
}

func TestConsLogger(t *testing.T) {
	buf := bytes.Buffer{}
	golog.SetOutput(&buf)

	consLogger(5, "^Error.*!$", 3*time.Second)

	exp := "[ERROR] plugin/errors: 5 errors like '^Error.*!$' occured in last 3s"
	act := buf.String()
	if !strings.Contains(act, exp) {
		t.Errorf("Unexpected log message, expected to contain %q, actual %q", exp, act)
	}
}

func TestLogPattern(t *testing.T) {
	h := &errorHandler{
		patterns: []*pattern{{
			count:   4,
			period:  2 * time.Second,
			pattern: regexp.MustCompile("^Error.*!$"),
		}},
		cLogger: testConsLogger,
	}
	loggedData = loggedData[:0]

	h.logPattern(0)
	expLen := 1
	if len(loggedData) != expLen {
		t.Errorf("Unexpected number of logged messages, expected %d, actual %d",
			expLen, len(loggedData))
	}
	expCnt := uint32(4)
	if loggedData[0].cnt != expCnt {
		t.Errorf("Unexpected 'count' in logged message, expected %d, actual %d",
			expCnt, loggedData[0].cnt)
	}
	expPat := "^Error.*!$"
	actPat := loggedData[0].pat
	if actPat != expPat {
		t.Errorf("Unexpected 'pattern' in logged message, expected %s, actual %s",
			expPat, actPat)
	}
	expPer := "2s"
	actPer := loggedData[0].dur.String()
	if actPer != expPer {
		t.Errorf("Unexpected 'period' in logged message, expected %s, actual %s",
			expPer, actPer)
	}

	h.logPattern(0)
	if len(loggedData) != expLen {
		t.Errorf("Unexpected number of logged messages, expected %d, actual %d",
			expLen, len(loggedData))
	}
}

func TestInc(t *testing.T) {
	h := &errorHandler{
		stopFlag: 1,
		patterns: []*pattern{{
			period:  2 * time.Second,
			pattern: regexp.MustCompile("^Error.*!$"),
		}},
		cLogger: testConsLogger,
	}

	ret := h.inc(0)
	if ret {
		t.Error("Unexpected return value, expected false, actual true")
	}

	h.stopFlag = 0
	ret = h.inc(0)
	if !ret {
		t.Error("Unexpected return value, expected true, actual false")
	}

	expCnt := uint32(1)
	actCnt := atomic.LoadUint32(&h.patterns[0].count)
	if actCnt != expCnt {
		t.Errorf("Unexpected 'count', expected %d, actual %d", expCnt, actCnt)
	}

	t1 := h.patterns[0].timer()
	if t1 == nil {
		t.Error("Unexpected 'timer', expected not nil")
	}

	ret = h.inc(0)
	if !ret {
		t.Error("Unexpected return value, expected true, actual false")
	}

	expCnt = uint32(2)
	actCnt = atomic.LoadUint32(&h.patterns[0].count)
	if actCnt != expCnt {
		t.Errorf("Unexpected 'count', expected %d, actual %d", expCnt, actCnt)
	}

	t2 := h.patterns[0].timer()
	if t2 != t1 {
		t.Error("Unexpected 'timer', expected the same")
	}

	ret = t1.Stop()
	if !ret {
		t.Error("Timer was unexpectedly stopped before")
	}
	ret = t2.Stop()
	if ret {
		t.Error("Timer was unexpectedly not stopped before")
	}
}

func TestStop(t *testing.T) {
	h := &errorHandler{
		patterns: []*pattern{{
			period:  2 * time.Second,
			pattern: regexp.MustCompile("^Error.*!$"),
		}},
		cLogger: testConsLogger,
	}
	loggedData = loggedData[:0]

	h.inc(0)
	h.inc(0)
	h.inc(0)
	expCnt := uint32(3)
	actCnt := atomic.LoadUint32(&h.patterns[0].count)
	if actCnt != expCnt {
		t.Fatalf("Unexpected initial 'count', expected %d, actual %d", expCnt, actCnt)
	}

	h.stop()

	expCnt = uint32(0)
	actCnt = atomic.LoadUint32(&h.patterns[0].count)
	if actCnt != expCnt {
		t.Errorf("Unexpected 'count', expected %d, actual %d", expCnt, actCnt)
	}

	expStop := uint32(1)
	actStop := h.stopFlag
	if actStop != expStop {
		t.Errorf("Unexpected 'stop', expected %d, actual %d", expStop, actStop)
	}

	t1 := h.patterns[0].timer()
	if t1 == nil {
		t.Error("Unexpected 'timer', expected not nil")
	} else if t1.Stop() {
		t.Error("Timer was unexpectedly not stopped before")
	}

	expLen := 1
	if len(loggedData) != expLen {
		t.Errorf("Unexpected number of logged messages, expected %d, actual %d",
			expLen, len(loggedData))
	}
	expCnt = uint32(3)
	actCnt = loggedData[0].cnt
	if actCnt != expCnt {
		t.Errorf("Unexpected 'count' in logged message, expected %d, actual %d",
			expCnt, actCnt)
	}
}

func TestServeDNSConcurrent(t *testing.T) {
	eg := newErrorGenerator()
	rep := &errorReport{}

	h := &errorHandler{
		patterns: []*pattern{
			{
				period:  3 * time.Nanosecond,
				pattern: regexp.MustCompile("Failed"),
			},
			{
				period:  2 * time.Nanosecond,
				pattern: regexp.MustCompile("down$"),
			},
		},
		cLogger: func(cnt uint32, pattern string, p time.Duration) {
			rep.incLoggerCnt()
			if pattern == "Failed" {
				rep.incConsolidated(Err0, cnt)
			} else if pattern == "down$" {
				rep.incConsolidated(Err1, cnt)
			}
		},
		eLogger: func(code int, n, t, e string) {
			switch e {
			case ErrStr0:
				rep.incPassed(Err0)
			case ErrStr1:
				rep.incPassed(Err1)
			case ErrStr2:
				rep.incPassed(Err2)
			case ErrStr3:
				rep.incPassed(Err3)
			}
		},
		Next: eg,
	}

	ctx := context.TODO()
	r := new(dns.Msg)
	w := &test.ResponseWriter{}

	var wg sync.WaitGroup
	runnersCnt := 9
	runner := func() {
		defer wg.Done()
		for !eg.done() {
			h.ServeDNS(ctx, w, r)
		}
	}
	wg.Add(runnersCnt)
	for ; runnersCnt > 0; runnersCnt-- {
		go runner()
	}

	stopCalled := false
	for !eg.done() {
		if !stopCalled &&
			atomic.LoadUint32(&rep.s[Err0]) > ErrCnt0/2 &&
			atomic.LoadUint32(&rep.s[Err1]) > ErrCnt1/2 {
			h.stop()
			stopCalled = true
		}
		h.ServeDNS(ctx, w, r)
	}
	if !stopCalled {
		h.stop()
	}

	wg.Wait()
	time.Sleep(time.Millisecond)

	if rep.loggerCnt() < 3 {
		t.Errorf("Perhaps logger was never called by timer")
	}
	if rep.consolidated(Err0) == 0 {
		t.Errorf("Err0 was never reported by consLogger")
	}
	if rep.consolidated(Err1) == 0 {
		t.Errorf("Err1 was never reported by consLogger")
	}
	if rep.consolidated(Err0)+rep.passed(Err0) != ErrCnt0 {
		t.Errorf("Unexpected Err0 count, expected %d, reported by consLogger %d and by ServeDNS %d",
			ErrCnt0, rep.consolidated(Err0), rep.passed(Err0))
	}
	if rep.consolidated(Err1)+rep.passed(Err1) != ErrCnt1 {
		t.Errorf("Unexpected Err1 count, expected %d, reported by consLogger %d and by ServeDNS %d",
			ErrCnt1, rep.consolidated(Err1), rep.passed(Err1))
	}
	if rep.passed(Err2) != ErrCnt2 {
		t.Errorf("Unexpected Err2 count, expected %d, reported by ServeDNS %d",
			ErrCnt2, rep.passed(Err2))
	}
	if rep.passed(Err3) != ErrCnt3 {
		t.Errorf("Unexpected Err3 count, expected %d, reported by ServeDNS %d",
			ErrCnt3, rep.passed(Err3))
	}
}

type logData struct {
	cnt uint32
	pat string
	dur time.Duration
}

var loggedData []logData

func testConsLogger(cnt uint32, pattern string, p time.Duration) {
	loggedData = append(loggedData, logData{cnt, pattern, p})
}

// error generator
const (
	Err0 = iota
	Err1
	Err2
	Err3

	ErrStr0 = "Failed to connect"
	ErrStr1 = "Upstream is down"
	ErrStr2 = "Access denied"
	ErrStr3 = "Yet another error"

	ErrCnt0 = 120000
	ErrCnt1 = 130000
	ErrCnt2 = 150000
	ErrCnt3 = 100000
)

type errorBunch struct {
	cnt int32
	err error
}

type errorGenerator struct {
	errors   [4]errorBunch
	doneFlag uint32
}

func newErrorGenerator() *errorGenerator {
	rand.Seed(time.Now().UnixNano())

	return &errorGenerator{
		errors: [4]errorBunch{
			{ErrCnt0, fmt.Errorf(ErrStr0)},
			{ErrCnt1, fmt.Errorf(ErrStr1)},
			{ErrCnt2, fmt.Errorf(ErrStr2)},
			{ErrCnt3, fmt.Errorf(ErrStr3)},
		},
	}
}

func (sh *errorGenerator) done() bool {
	return atomic.LoadUint32(&sh.doneFlag) > 0
}

func (sh *errorGenerator) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	i := rand.Int()
	for c := 0; c < 4; c++ {
		errInd := (i + c) & 3
		if atomic.AddInt32(&sh.errors[errInd].cnt, -1) >= 0 {
			return 0, sh.errors[errInd].err
		}
		atomic.AddInt32(&sh.errors[errInd].cnt, 1)
	}
	atomic.StoreUint32(&sh.doneFlag, 1)
	return 0, nil
}

func (sh *errorGenerator) Name() string { return "errorGenerator" }

// error report
type errorReport struct {
	s [4]uint32
	p [4]uint32
	l uint32
}

func (er *errorReport) consolidated(i int) uint32 {
	return atomic.LoadUint32(&er.s[i])
}

func (er *errorReport) incConsolidated(i int, cnt uint32) {
	atomic.AddUint32(&er.s[i], cnt)
}

func (er *errorReport) passed(i int) uint32 {
	return atomic.LoadUint32(&er.p[i])
}

func (er *errorReport) incPassed(i int) {
	atomic.AddUint32(&er.p[i], 1)
}

func (er *errorReport) loggerCnt() uint32 {
	return atomic.LoadUint32(&er.l)
}

func (er *errorReport) incLoggerCnt() {
	atomic.AddUint32(&er.l, 1)
}

func genErrorHandler(rcode int, err error) plugin.Handler {
	return plugin.HandlerFunc(func(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
		return rcode, err
	})
}
