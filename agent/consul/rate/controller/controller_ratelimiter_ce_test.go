// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package controller

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/consul/controller"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
)

// TestReconcileEntry_Success tests the successful reconciliation of a rate limit entry.
func TestReconcileEntry_Success(t *testing.T) {
	logger := hclog.NewNullLogger()

	// Create a sample rate limit config entry
	cfg := &structs.GlobalRateLimitConfigEntry{
		Kind: structs.RateLimit,
		Name: "global",
	}

	mockReadEntry := NewMockceReadEntryFunc()
	mockReadEntry.On("Execute", structs.RateLimit, "global").
		Return(uint64(1), cfg, nil)

	// Mock the updater
	mockUpdater := NewMockceUpdater()
	mockUpdater.On("UpdateGlobalRateLimitConfig", cfg).Return()

	req := controller.Request{
		Kind: structs.RateLimit,
		Name: "global",
	}

	// Call reconcileEntry
	err := reconcileEntry(mockReadEntry.Execute, logger, context.Background(), req, mockUpdater)

	require.NoError(t, err)
	mockReadEntry.AssertCalled(t, "Execute", structs.RateLimit, "global")
	mockUpdater.AssertCalled(t, "UpdateGlobalRateLimitConfig", cfg)
	mockUpdater.AssertNumberOfCalls(t, "UpdateGlobalRateLimitConfig", 1)
}

// TestReconcileEntry_EntryNotFound tests reconciliation when the config entry is not found.
func TestReconcileEntry_EntryNotFound(t *testing.T) {
	logger := hclog.NewNullLogger()

	// Mock the readEntry function returning nil (entry not found)
	mockReadEntry := NewMockceReadEntryFunc()
	mockReadEntry.On("Execute", structs.RateLimit, "non-existent").
		Return(uint64(0), nil, nil)

	// Mock the updater
	mockUpdater := NewMockceUpdater()
	mockUpdater.On("UpdateGlobalRateLimitConfig", mock.MatchedBy(func(cfg *structs.GlobalRateLimitConfigEntry) bool {
		return cfg == nil
	})).Return()

	req := controller.Request{
		Kind: structs.RateLimit,
		Name: "non-existent",
	}

	// Call reconcileEntry
	err := reconcileEntry(mockReadEntry.Execute, logger, context.Background(), req, mockUpdater)

	require.NoError(t, err)
	mockReadEntry.AssertCalled(t, "Execute", structs.RateLimit, "non-existent")
	mockUpdater.AssertNumberOfCalls(t, "UpdateGlobalRateLimitConfig", 1)
}

// TestReconcileEntry_ReadError tests reconciliation when reading the config entry fails.
func TestReconcileEntry_ReadError(t *testing.T) {
	logger := hclog.NewNullLogger()

	// Mock the readEntry function returning an error
	expectedErr := errors.New("failed to read from store")

	mockReadEntry := NewMockceReadEntryFunc()
	mockReadEntry.On("Execute", structs.RateLimit, "global").
		Return(uint64(0), nil, expectedErr)

	// Mock the updater - should not be called
	mockUpdater := NewMockceUpdater()

	req := controller.Request{
		Kind: structs.RateLimit,
		Name: "global",
	}

	// Call reconcileEntry
	err := reconcileEntry(mockReadEntry.Execute, logger, context.Background(), req, mockUpdater)

	require.Error(t, err)
	require.Equal(t, expectedErr, err)
	mockReadEntry.AssertCalled(t, "Execute", structs.RateLimit, "global")
	mockUpdater.AssertNotCalled(t, "UpdateGlobalRateLimitConfig")
}

// TestReconcileEntry_InvalidCast tests reconciliation when the entry cannot be cast to GlobalRateLimitConfigEntry.
func TestReconcileEntry_InvalidCast(t *testing.T) {
	logger := hclog.NewNullLogger()

	// Create a different type of config entry (not GlobalRateLimitConfigEntry)
	// We can use a simple string to simulate a non-matching type
	invalidEntry := &structs.ProxyConfigEntry{} // Different config entry type

	mockReadEntry := NewMockceReadEntryFunc()
	mockReadEntry.On("Execute", structs.RateLimit, "global").
		Return(uint64(1), invalidEntry, nil)

	// Mock the updater - should not be called when cast fails
	mockUpdater := NewMockceUpdater()

	req := controller.Request{
		Kind: structs.RateLimit,
		Name: "global",
	}

	// Call reconcileEntry
	err := reconcileEntry(mockReadEntry.Execute, logger, context.Background(), req, mockUpdater)

	require.NoError(t, err)
	mockReadEntry.AssertCalled(t, "Execute", structs.RateLimit, "global")
	mockUpdater.AssertNotCalled(t, "UpdateGlobalRateLimitConfig")
}

// TestReconcilerReconcile_UnknownKind tests the Reconcile method with an unknown kind (should return nil).
func TestReconcilerReconcile_UnknownKind(t *testing.T) {
	mockReadEntry := NewMockceReadEntryFunc()
	mockUpdater := NewMockceUpdater()

	reconciler := &rateLimiterReconciler{
		readEntry: mockReadEntry.Execute,
		logger:    hclog.NewNullLogger(),
		updater:   mockUpdater,
	}

	req := controller.Request{
		Kind: "unknown-kind",
		Name: "test-config",
	}

	err := reconciler.Reconcile(context.Background(), req)

	require.NoError(t, err)
	mockReadEntry.AssertNotCalled(t, "Execute")
	mockUpdater.AssertNotCalled(t, "UpdateGlobalRateLimitConfig")
}

// TestNewRateLimiterController_InitializesController tests that NewRateLimiterController properly initializes the controller.
func TestNewRateLimiterController_InitializesController(t *testing.T) {
	mockReadEntry := NewMockceReadEntryFunc()
	publisher := stream.NewEventPublisher(1 * time.Millisecond)
	logger := hclog.NewNullLogger()
	mockUpdater := NewMockceUpdater()

	ctrl := NewRateLimiterController(mockReadEntry.Execute, publisher, logger, mockUpdater)

	// Verify that the controller is not nil
	require.NotNil(t, ctrl)
}
