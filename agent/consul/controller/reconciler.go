// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/consul/acl"
)

// Request contains the information necessary to reconcile a config entry.
// This includes only the information required to uniquely identify the
// config entry.
type Request struct {
	Kind string
	Name string
	Meta *acl.EnterpriseMeta
}

// Key satisfies the queue.ItemType interface. It returns a string which will be
// used to de-duplicate requests in the queue.
func (r Request) Key() string {
	return fmt.Sprintf(
		`kind=%q,name=%q,part=%q,ns=%q`,
		r.Kind,
		r.Name,
		r.Meta.PartitionOrDefault(),
		r.Meta.NamespaceOrDefault(),
	)
}

// RequeueAfterError is an error that allows a Reconciler to override the
// exponential backoff behavior of the Controller, rather than applying
// the backoff algorithm, returning a RequeueAfterError will cause the
// Controller to reschedule the Request at a given time in the future.
type RequeueAfterError time.Duration

// Error implements the error interface.
func (r RequeueAfterError) Error() string {
	return fmt.Sprintf("requeue at %s", time.Duration(r))
}

// RequeueAfter constructs a RequeueAfterError with the given duration
// setting.
func RequeueAfter(after time.Duration) error {
	return RequeueAfterError(after)
}

// RequeueNow constructs a RequeueAfterError that reschedules the Request
// immediately.
func RequeueNow() error {
	return RequeueAfterError(0)
}

// Reconciler is the main implementation interface for Controllers. A Reconciler
// receives any change notifications for config entries that the controller is subscribed
// to and processes them with its Reconcile function.
type Reconciler interface {
	// Reconcile performs a reconciliation on the config entry referred to by the Request.
	// The Controller will requeue the Request to be processed again if an error is non-nil.
	// If no error is returned, the Request will be removed from the working queue.
	Reconcile(context.Context, Request) error
}
