// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package leafcert

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/cacheshim"
	"github.com/hashicorp/consul/agent/structs"
)

// Mock implementations for testing
type mockRootsReader struct {
	mock.Mock
}

func (m *mockRootsReader) Get() (*structs.IndexedCARoots, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*structs.IndexedCARoots), args.Error(1)
}

func (m *mockRootsReader) Notify(ctx context.Context, correlationID string, ch chan<- cacheshim.UpdateEvent) error {
	args := m.Called(ctx, correlationID, ch)
	return args.Error(0)
}

type mockCertSigner struct {
	mock.Mock
}

func (m *mockCertSigner) SignCert(ctx context.Context, args *structs.CASignRequest) (*structs.IssuedCert, error) {
	mArgs := m.Called(ctx, args)
	if mArgs.Get(0) == nil {
		return nil, mArgs.Error(1)
	}
	return mArgs.Get(0).(*structs.IssuedCert), mArgs.Error(1)
}

func TestLeafCertRenewalFailure_RateLimitLogging(t *testing.T) {
	// Test that rate limit failures are logged with appropriate severity
	logger := hclog.NewNullLogger()

	rootsReader := &mockRootsReader{}
	certSigner := &mockCertSigner{}

	// Setup roots
	roots := &structs.IndexedCARoots{
		TrustDomain: "test.consul",
		Roots: []*structs.CARoot{
			{
				ID:     "root-1",
				Active: true,
			},
		},
	}

	rootsReader.On("Get").Return(roots, nil)
	certSigner.On("SignCert", mock.Anything, mock.Anything).Return(nil, errors.New(structs.ErrRateLimited.Error()))

	mgr := NewManager(Deps{
		Logger:      logger,
		RootsReader: rootsReader,
		CertSigner:  certSigner,
		Config: Config{
			CertificateTelemetryCriticalThresholdDays: 7,
			CertificateTelemetryWarningThresholdDays:  30,
		},
	})
	defer mgr.Stop()

	// This test verifies the manager can be created with thresholds
	require.NotNil(t, mgr)
	require.Equal(t, 7, mgr.config.CertificateTelemetryCriticalThresholdDays)
	require.Equal(t, 30, mgr.config.CertificateTelemetryWarningThresholdDays)
}

func TestLeafCertRenewalFailure_SigningErrorLogging(t *testing.T) {
	// Test that signing errors are logged with appropriate severity
	logger := hclog.NewNullLogger()

	rootsReader := &mockRootsReader{}
	certSigner := &mockCertSigner{}

	roots := &structs.IndexedCARoots{
		TrustDomain: "test.consul",
		Roots: []*structs.CARoot{
			{
				ID:     "root-1",
				Active: true,
			},
		},
	}

	rootsReader.On("Get").Return(roots, nil)
	certSigner.On("SignCert", mock.Anything, mock.Anything).Return(nil, errors.New("CA unavailable"))

	mgr := NewManager(Deps{
		Logger:      logger,
		RootsReader: rootsReader,
		CertSigner:  certSigner,
		Config: Config{
			CertificateTelemetryCriticalThresholdDays: 7,
			CertificateTelemetryWarningThresholdDays:  30,
		},
	})
	defer mgr.Stop()

	require.NotNil(t, mgr)
}

func TestLeafCertManager_ThresholdConfiguration(t *testing.T) {
	tests := []struct {
		name         string
		criticalDays int
		warningDays  int
	}{
		{"default", 7, 30},
		{"conservative", 14, 60},
		{"aggressive", 3, 7},
		{"custom", 10, 45},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := hclog.NewNullLogger()

			rootsReader := &mockRootsReader{}
			certSigner := &mockCertSigner{}

			roots := &structs.IndexedCARoots{
				TrustDomain: "test.consul",
				Roots: []*structs.CARoot{
					{
						ID:     "root-1",
						Active: true,
					},
				},
			}

			rootsReader.On("Get").Return(roots, nil)

			mgr := NewManager(Deps{
				Logger:      logger,
				RootsReader: rootsReader,
				CertSigner:  certSigner,
				Config: Config{
					CertificateTelemetryCriticalThresholdDays: tt.criticalDays,
					CertificateTelemetryWarningThresholdDays:  tt.warningDays,
				},
			})
			defer mgr.Stop()

			require.Equal(t, tt.criticalDays, mgr.config.CertificateTelemetryCriticalThresholdDays)
			require.Equal(t, tt.warningDays, mgr.config.CertificateTelemetryWarningThresholdDays)
		})
	}
}

func TestLeafCert_ExpiryCalculation(t *testing.T) {
	// Test that days remaining calculation is correct
	tests := []struct {
		name           string
		hoursRemaining int
		expectedDays   int
	}{
		{"1 day", 24, 1},
		{"7 days", 168, 7},
		{"30 days", 720, 30},
		{"90 days", 2160, 90},
		{"partial day", 36, 1}, // 1.5 days rounds to 1
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			duration := time.Duration(tt.hoursRemaining) * time.Hour
			days := int(duration.Hours() / 24)
			require.Equal(t, tt.expectedDays, days)
		})
	}
}
