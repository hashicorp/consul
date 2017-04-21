package api

import (
	"fmt"
	"testing"

	"github.com/hashicorp/consul/testutil/retry"
)

func TestEvent_FireList(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	event := c.Event()

	params := &UserEvent{Name: "foo"}
	id, meta, err := event.Fire(params, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if meta.RequestTime == 0 {
		t.Fatalf("bad: %v", meta)
	}

	if id == "" {
		t.Fatalf("invalid: %v", id)
	}

	var events []*UserEvent
	var qm *QueryMeta
	retry.Fatal(t, func() error {
		events, qm, err = event.List("", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(events) == 0 {
			return fmt.Errorf("no events")
		}
		return nil
	})

	if events[len(events)-1].ID != id {
		t.Fatalf("bad: %#v", events)
	}

	if qm.LastIndex != event.IDToIndex(id) {
		t.Fatalf("Bad: %#v", qm)
	}
}
