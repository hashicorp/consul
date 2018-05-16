package gls

import (
	"testing"
	"sync"
)

func TestGls(t *testing.T) {
	wg := sync.WaitGroup{}
	wg.Add(1)
	go WithGls(func() {
		if nil != Get("hello") {
			t.Fail()
		}
		Set("hello", "world")
		if "world" != Get("hello") {
			t.Fail()
		}
		if !IsGlsEnabled(GoID()) {
			t.Fail()
		}
		wg.Done()
	})()
	wg.Wait()
	if IsGlsEnabled(GoID()) {
		t.Fail()
	}
	if nil != Get("hello") {
		t.Fail()
	}
	//SetIndex("hello", "world") // will panic
}

func TestNestedGls(t *testing.T) {
	wg := sync.WaitGroup{}
	wg.Add(1)
	go WithGls(func() {
		Set("hello", "world")
		go WithGls(func() {
			if "world" != Get("hello") {
				t.Fail()
			}
			wg.Done()
		})()
	})()
	wg.Wait()
}
