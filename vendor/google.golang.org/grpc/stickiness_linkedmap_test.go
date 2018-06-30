/*
 *
 * Copyright 2018 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package grpc

import (
	"container/list"
	"fmt"
	"reflect"
	"testing"
)

var linkedMapTestData = make([]*stickyStoreEntry, 5)

func TestLinkedMapPutGet(t *testing.T) {
	m := newLinkedMap()
	m.put("one", linkedMapTestData[0])
	if got, ok := m.get("one"); !ok || got != linkedMapTestData[0] {
		t.Errorf("m.Get(%v) = %v, %v, want %v, true", 1, got, ok, "one")
	}
	m.put("two", linkedMapTestData[1])
	if got, ok := m.get("two"); !ok || got != linkedMapTestData[1] {
		t.Errorf("m.Get(%v) = %v, %v, want %v, true", 2, got, ok, "two")
	}
	m.put("oneone", linkedMapTestData[4])
	if got, ok := m.get("one"); !ok || got != linkedMapTestData[4] {
		t.Errorf("m.Get(%v) = %v, %v, want %v, true", 1, got, ok, "oneone")
	}
}

func TestLinkedMapRemove(t *testing.T) {
	m := newLinkedMap()
	m.put("one", linkedMapTestData[0])
	if got, ok := m.get("one"); !ok || got != linkedMapTestData[0] {
		t.Errorf("m.Get(%v) = %v, %v, want %v, true", 1, got, ok, "one")
	}
	m.put("two", linkedMapTestData[1])
	if got, ok := m.get("two"); !ok || got != linkedMapTestData[1] {
		t.Errorf("m.Get(%v) = %v, %v, want %v, true", 2, got, ok, "two")
	}

	if got, ok := m.remove("one"); !ok || got != linkedMapTestData[0] {
		t.Errorf("m.Remove(%v) = %v, %v, want %v, true", 1, got, ok, "one")
	}
	if got, ok := m.get("one"); ok {
		t.Errorf("m.Get(%v) = %v, %v, want _, false", 1, got, ok)
	}
	// 2 should still in the map.
	if got, ok := m.get("two"); !ok || got != linkedMapTestData[1] {
		t.Errorf("m.Get(%v) = %v, %v, want %v, true", 2, got, ok, "two")
	}
}

func TestLinkedMapLen(t *testing.T) {
	m := newLinkedMap()
	if got := m.len(); got != 0 {
		t.Errorf("m.Len() = %v, want %v", got, 0)
	}
	m.put("one", linkedMapTestData[0])
	if got := m.len(); got != 1 {
		t.Errorf("m.Len() = %v, want %v", got, 1)
	}
	m.put("two", linkedMapTestData[1])
	if got := m.len(); got != 2 {
		t.Errorf("m.Len() = %v, want %v", got, 2)
	}
	m.put("one", linkedMapTestData[4])
	if got := m.len(); got != 2 {
		t.Errorf("m.Len() = %v, want %v", got, 2)
	}

	// Internal checks.
	if got := len(m.m); got != 2 {
		t.Errorf("len(m.m) = %v, want %v", got, 2)
	}
	if got := m.l.Len(); got != 2 {
		t.Errorf("m.l.Len() = %v, want %v", got, 2)
	}
}

func TestLinkedMapClear(t *testing.T) {
	m := newLinkedMap()
	m.put("one", linkedMapTestData[0])
	if got, ok := m.get("one"); !ok || got != linkedMapTestData[0] {
		t.Errorf("m.Get(%v) = %v, %v, want %v, true", 1, got, ok, "one")
	}
	m.put("two", linkedMapTestData[1])
	if got, ok := m.get("two"); !ok || got != linkedMapTestData[1] {
		t.Errorf("m.Get(%v) = %v, %v, want %v, true", 2, got, ok, "two")
	}

	m.clear()
	if got, ok := m.get("one"); ok {
		t.Errorf("m.Get(%v) = %v, %v, want _, false", 1, got, ok)
	}
	if got, ok := m.get("two"); ok {
		t.Errorf("m.Get(%v) = %v, %v, want _, false", 2, got, ok)
	}
	if got := m.len(); got != 0 {
		t.Errorf("m.Len() = %v, want %v", got, 0)
	}
}

func TestLinkedMapRemoveOldest(t *testing.T) {
	m := newLinkedMap()
	m.put("one", linkedMapTestData[0])
	m.put("two", linkedMapTestData[1])
	m.put("three", linkedMapTestData[2])
	m.put("four", linkedMapTestData[3])
	if got, ok := m.get("one"); !ok || got != linkedMapTestData[0] {
		t.Errorf("m.Get(%v) = %v, %v, want %v, true", 1, got, ok, "one")
	}
	if got, ok := m.get("two"); !ok || got != linkedMapTestData[1] {
		t.Errorf("m.Get(%v) = %v, %v, want %v, true", 2, got, ok, "two")
	}
	if got, ok := m.get("three"); !ok || got != linkedMapTestData[2] {
		t.Errorf("m.Get(%v) = %v, %v, want %v, true", 3, got, ok, "three")
	}
	if got, ok := m.get("four"); !ok || got != linkedMapTestData[3] {
		t.Errorf("m.Get(%v) = %v, %v, want %v, true", 4, got, ok, "four")
	}

	if err := checkListOrdered(m.l, []string{"one", "two", "three", "four"}); err != nil {
		t.Fatalf("m.l is not expected: %v", err)
	}

	m.put("three", linkedMapTestData[2])
	if err := checkListOrdered(m.l, []string{"one", "two", "four", "three"}); err != nil {
		t.Fatalf("m.l is not expected: %v", err)
	}
	m.put("four", linkedMapTestData[3])
	if err := checkListOrdered(m.l, []string{"one", "two", "three", "four"}); err != nil {
		t.Fatalf("m.l is not expected: %v", err)
	}

	m.removeOldest()
	if got, ok := m.get("one"); ok {
		t.Errorf("m.Get(%v) = %v, %v, want _, false", 1, got, ok)
	}
	if err := checkListOrdered(m.l, []string{"two", "three", "four"}); err != nil {
		t.Fatalf("m.l is not expected: %v", err)
	}

	m.get("two") // 2 is refreshed, 3 becomes the oldest
	if err := checkListOrdered(m.l, []string{"three", "four", "two"}); err != nil {
		t.Fatalf("m.l is not expected: %v", err)
	}

	m.removeOldest()
	if got, ok := m.get("three"); ok {
		t.Errorf("m.Get(%v) = %v, %v, want _, false", 3, got, ok)
	}
	// 2 and 4 are still in map.
	if got, ok := m.get("two"); !ok || got != linkedMapTestData[1] {
		t.Errorf("m.Get(%v) = %v, %v, want %v, true", 2, got, ok, "two")
	}
	if got, ok := m.get("four"); !ok || got != linkedMapTestData[3] {
		t.Errorf("m.Get(%v) = %v, %v, want %v, true", 4, got, ok, "four")
	}
}

func checkListOrdered(l *list.List, want []string) error {
	got := make([]string, 0, len(want))
	for p := l.Front(); p != nil; p = p.Next() {
		got = append(got, p.Value.(*linkedMapKVPair).key)
	}
	if !reflect.DeepEqual(got, want) {
		return fmt.Errorf("list elements: %v, want %v", got, want)
	}
	return nil
}
