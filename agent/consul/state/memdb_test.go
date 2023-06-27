// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package state

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/hashicorp/go-memdb"
)

func testValidSchema() *memdb.DBSchema {
	return &memdb.DBSchema{
		Tables: map[string]*memdb.TableSchema{
			"main": {
				Name: "main",
				Indexes: map[string]*memdb.IndexSchema{
					"id": {
						Name:    "id",
						Unique:  true,
						Indexer: &memdb.StringFieldIndex{Field: "ID"},
					},
					"foo": {
						Name:    "foo",
						Indexer: &memdb.StringFieldIndex{Field: "Foo"},
					},
				},
			},
		},
	}
}

type TestObject struct {
	ID  string
	Foo string
}

// This test verify that the new data in a TXN is commited at the time that publishFunc is called.
// To do so, the publish func is mocked, a read on ch1 means that publish is called and blocked,
// ch2 permit to control the publish func and unblock it when receiving a signal.
func Test_txn_Commit(t *testing.T) {
	db, err := memdb.NewMemDB(testValidSchema())
	require.NoError(t, err)
	publishFunc := mockPublishFuncType{}
	tx := txn{
		Txn:     db.Txn(true),
		Index:   0,
		publish: publishFunc.Execute,
	}
	ch1 := make(chan struct{})
	ch2 := make(chan struct{})
	getCh := make(chan memdb.ResultIterator)
	group := errgroup.Group{}
	group.Go(func() error {
		after := time.After(2 * time.Second)
		select {
		case <-ch1:
			tx2 := txn{
				Txn:     db.Txn(false),
				Index:   0,
				publish: publishFunc.Execute,
			}
			get, err := tx2.Get("main", "id")
			if err != nil {
				return err
			}
			close(ch2)
			getCh <- get
		case <-after:
			close(ch2)
			return fmt.Errorf("test timed out")
		}
		return nil
	})

	publishFunc.On("Execute", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		close(ch1)
		<-ch2
	}).Return(nil)

	err = tx.Insert("main", TestObject{ID: "1", Foo: "foo"})
	require.NoError(t, err)
	err = tx.Commit()
	require.NoError(t, err)
	get := <-getCh
	require.NotNil(t, get)
	next := get.Next()
	require.NotNil(t, next)

	val := next.(TestObject)
	require.Equal(t, val.ID, "1")
	require.Equal(t, val.Foo, "foo")

	err = group.Wait()
	require.NoError(t, err)

}
