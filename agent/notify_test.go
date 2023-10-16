// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"testing"
)

// Used to be defined in NotifyGroup.WaitCh but was only used in tests and prone
// to leaking memory if anything real did use it because there is no way to
// clear the chan later.
func testWaitCh(t *testing.T, grp *NotifyGroup) chan struct{} {
	ch := make(chan struct{}, 1)
	grp.Wait(ch)
	return ch
}

func TestNotifyGroup(t *testing.T) {
	grp := &NotifyGroup{}

	ch1 := testWaitCh(t, grp)
	ch2 := testWaitCh(t, grp)

	select {
	case <-ch1:
		t.Fatalf("should block")
	default:
	}
	select {
	case <-ch2:
		t.Fatalf("should block")
	default:
	}

	grp.Notify()

	select {
	case <-ch1:
	default:
		t.Fatalf("should not block")
	}
	select {
	case <-ch2:
	default:
		t.Fatalf("should not block")
	}

	// Should be unregistered
	ch3 := testWaitCh(t, grp)
	grp.Notify()

	select {
	case <-ch1:
		t.Fatalf("should block")
	default:
	}
	select {
	case <-ch2:
		t.Fatalf("should block")
	default:
	}
	select {
	case <-ch3:
	default:
		t.Fatalf("should not block")
	}
}

func TestNotifyGroup_Clear(t *testing.T) {
	grp := &NotifyGroup{}

	ch1 := testWaitCh(t, grp)
	grp.Clear(ch1)

	grp.Notify()

	// Should not get anything
	select {
	case <-ch1:
		t.Fatalf("should not get message")
	default:
	}
}
