package errors

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	golog "log"
	"math/rand"
	"regexp"
	"strconv"
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
	em := errorHandler{}

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

func TestLogPattern(t *testing.T) {
	buf := bytes.Buffer{}
	golog.SetOutput(&buf)

	h := &errorHandler{
		patterns: []*pattern{{
			count:   4,
			period:  2 * time.Second,
			pattern: regexp.MustCompile("^error.*!$"),
		}},
	}
	h.logPattern(0)

	expLog := "4 errors like '^error.*!$' occurred in last 2s"
	if log := buf.String(); !strings.Contains(log, expLog) {
		t.Errorf("Expected log %q, but got %q", expLog, log)
	}
}

func TestInc(t *testing.T) {
	h := &errorHandler{
		stopFlag: 1,
		patterns: []*pattern{{
			period:  2 * time.Second,
			pattern: regexp.MustCompile("^error.*!$"),
		}},
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
	buf := bytes.Buffer{}
	golog.SetOutput(&buf)

	h := &errorHandler{
		patterns: []*pattern{{
			period:  2 * time.Second,
			pattern: regexp.MustCompile("^error.*!$"),
		}},
	}

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

	expLog := "3 errors like '^error.*!$' occurred in last 2s"
	if log := buf.String(); !strings.Contains(log, expLog) {
		t.Errorf("Expected log %q, but got %q", expLog, log)
	}
}

const (
	pattern0 = "failed"
	pattern1 = "down$"
)

func TestServeDNSConcurrent(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	golog.SetOutput(buf)

	eg := newErrorGenerator()

	h := &errorHandler{
		patterns: []*pattern{
			{
				period:  3 * time.Nanosecond,
				pattern: regexp.MustCompile(pattern0),
			},
			{
				period:  2 * time.Nanosecond,
				pattern: regexp.MustCompile(pattern1),
			},
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
		for eg.progress() < 100 {
			h.ServeDNS(ctx, w, r)
		}
	}

	wg.Add(runnersCnt)
	for ; runnersCnt > 0; runnersCnt-- {
		go runner()
	}

	stopCalled := false
	for eg.progress() < 100 {
		if !stopCalled && eg.progress() > 50 {
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

	rep := makeReport(t, buf)

	if rep.c[Err0] == 0 {
		t.Errorf("Err0 was never consolidated")
	}
	if rep.c[Err1] == 0 {
		t.Errorf("Err1 was never consolidated")
	}
	if rep.c[Err0]+rep.r[Err0] != ErrCnt0 {
		t.Errorf("Unexpected Err0 count, expected %d, consolidated %d and reported %d",
			ErrCnt0, rep.c[Err0], rep.r[Err0])
	}
	if rep.c[Err1]+rep.r[Err1] != ErrCnt1 {
		t.Errorf("Unexpected Err1 count, expected %d, consolidated %d and reported %d",
			ErrCnt1, rep.c[Err1], rep.r[Err1])
	}
	if rep.r[Err2] != ErrCnt2 {
		t.Errorf("Unexpected Err2 count, expected %d, reported %d",
			ErrCnt2, rep.r[Err2])
	}
	if rep.r[Err3] != ErrCnt3 {
		t.Errorf("Unexpected Err3 count, expected %d, reported %d",
			ErrCnt3, rep.r[Err3])
	}
}

// error generator
const (
	Err0 = iota
	Err1
	Err2
	Err3

	ErrStr0 = "failed to connect"
	ErrStr1 = "upstream is down"
	ErrStr2 = "access denied"
	ErrStr3 = "yet another error"

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
	totalCnt int32
	totalLim int32
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
		totalLim: ErrCnt0 + ErrCnt1 + ErrCnt2 + ErrCnt3,
	}
}

// returns progress as a value from [0..100] range
func (sh *errorGenerator) progress() int32 {
	return atomic.LoadInt32(&sh.totalCnt) * 100 / sh.totalLim
}

func (sh *errorGenerator) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	i := rand.Int()
	for c := 0; c < 4; c++ {
		errInd := (i + c) & 3
		if atomic.AddInt32(&sh.errors[errInd].cnt, -1) >= 0 {
			atomic.AddInt32(&sh.totalCnt, 1)
			return 0, sh.errors[errInd].err
		}
		atomic.AddInt32(&sh.errors[errInd].cnt, 1)
	}
	return 0, nil
}

func (sh *errorGenerator) Name() string { return "errorGenerator" }

// error report
type errorReport struct {
	c [4]uint32
	r [4]uint32
}

func makeReport(t *testing.T, b *bytes.Buffer) errorReport {
	logDump := b.String()
	r := errorReport{}

	re := regexp.MustCompile(`(\d+) errors like '(.*)' occurred`)
	m := re.FindAllStringSubmatch(logDump, -1)
	for _, sub := range m {
		ind := 0
		switch sub[2] {
		case pattern0:
			ind = Err0
		case pattern1:
			ind = Err1
		default:
			t.Fatalf("Unknown pattern found in log: %s", sub[0])
		}
		n, err := strconv.ParseUint(sub[1], 10, 32)
		if err != nil {
			t.Fatalf("Invalid number found in log: %s", sub[0])
		}
		r.c[ind] += uint32(n)
	}
	t.Logf("%d consolidated messages found", len(m))

	r.r[Err0] = uint32(strings.Count(logDump, ErrStr0))
	r.r[Err1] = uint32(strings.Count(logDump, ErrStr1))
	r.r[Err2] = uint32(strings.Count(logDump, ErrStr2))
	r.r[Err3] = uint32(strings.Count(logDump, ErrStr3))
	return r
}

func genErrorHandler(rcode int, err error) plugin.Handler {
	return plugin.HandlerFunc(func(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
		return rcode, err
	})
}
