// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package discovery

import (
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	testContext = Context{
		Token: "bar",
	}

	testErr = errors.New("test error")

	testIP = net.ParseIP("1.2.3.4")

	testPayload = QueryPayload{
		Name: "foo",
	}

	testResult = &Result{
		Node:    &Location{Address: "1.2.3.4"},
		Type:    ResultTypeNode, // This isn't correct for some test cases, but we are only asserting the right data fetcher functions are called
		Service: &Location{Name: "foo"},
	}
)

func TestQueryByName(t *testing.T) {

	type testCase struct {
		name                 string
		reqType              QueryType
		configureDataFetcher func(*testing.T, *MockCatalogDataFetcher)
		expectedResults      []*Result
		expectedError        error
	}

	run := func(t *testing.T, tc testCase) {

		fetcher := NewMockCatalogDataFetcher(t)
		tc.configureDataFetcher(t, fetcher)

		qp := NewQueryProcessor(fetcher)

		q := Query{
			QueryType:    tc.reqType,
			QueryPayload: testPayload,
		}

		results, err := qp.QueryByName(&q, testContext)
		if tc.expectedError != nil {
			require.Error(t, err)
			require.True(t, errors.Is(err, tc.expectedError))
			require.Nil(t, results)
			return
		}
		require.NoError(t, err)
		require.Equal(t, tc.expectedResults, results)
	}

	testCases := []testCase{
		{
			name:    "query node",
			reqType: QueryTypeNode,
			configureDataFetcher: func(t *testing.T, fetcher *MockCatalogDataFetcher) {

				fetcher.On("ValidateRequest", mock.Anything, mock.Anything).Return(nil)
				fetcher.On("NormalizeRequest", mock.Anything)
				fetcher.On("FetchNodes", mock.Anything, mock.Anything).Return([]*Result{testResult}, nil)
			},
			expectedResults: []*Result{testResult},
		},
		{
			name:    "query service",
			reqType: QueryTypeService,
			configureDataFetcher: func(t *testing.T, fetcher *MockCatalogDataFetcher) {

				fetcher.On("ValidateRequest", mock.Anything, mock.Anything).Return(nil)
				fetcher.On("NormalizeRequest", mock.Anything)
				fetcher.On("FetchEndpoints", mock.Anything, mock.Anything, mock.Anything).Return([]*Result{testResult}, nil)
			},
			expectedResults: []*Result{testResult},
		},
		{
			name:    "query connect",
			reqType: QueryTypeConnect,
			configureDataFetcher: func(t *testing.T, fetcher *MockCatalogDataFetcher) {

				fetcher.On("ValidateRequest", mock.Anything, mock.Anything).Return(nil)
				fetcher.On("NormalizeRequest", mock.Anything)
				fetcher.On("FetchEndpoints", mock.Anything, mock.Anything, mock.Anything).Return([]*Result{testResult}, nil)
			},
			expectedResults: []*Result{testResult},
		},
		{
			name:    "query ingress",
			reqType: QueryTypeIngress,
			configureDataFetcher: func(t *testing.T, fetcher *MockCatalogDataFetcher) {

				fetcher.On("ValidateRequest", mock.Anything, mock.Anything).Return(nil)
				fetcher.On("NormalizeRequest", mock.Anything)
				fetcher.On("FetchEndpoints", mock.Anything, mock.Anything, mock.Anything).Return([]*Result{testResult}, nil)
			},
			expectedResults: []*Result{testResult},
		},
		{
			name:    "query virtual ip",
			reqType: QueryTypeVirtual,
			configureDataFetcher: func(t *testing.T, fetcher *MockCatalogDataFetcher) {

				fetcher.On("ValidateRequest", mock.Anything, mock.Anything).Return(nil)
				fetcher.On("NormalizeRequest", mock.Anything)
				fetcher.On("FetchVirtualIP", mock.Anything, mock.Anything).Return(testResult, nil)
			},
			expectedResults: []*Result{testResult},
		},
		{
			name:    "query workload",
			reqType: QueryTypeWorkload,
			configureDataFetcher: func(t *testing.T, fetcher *MockCatalogDataFetcher) {

				fetcher.On("ValidateRequest", mock.Anything, mock.Anything).Return(nil)
				fetcher.On("NormalizeRequest", mock.Anything)
				fetcher.On("FetchWorkload", mock.Anything, mock.Anything).Return(testResult, nil)
			},
			expectedResults: []*Result{testResult},
		},
		{
			name:    "query prepared query",
			reqType: QueryTypePreparedQuery,
			configureDataFetcher: func(t *testing.T, fetcher *MockCatalogDataFetcher) {

				fetcher.On("ValidateRequest", mock.Anything, mock.Anything).Return(nil)
				fetcher.On("NormalizeRequest", mock.Anything)
				fetcher.On("FetchPreparedQuery", mock.Anything, mock.Anything).Return([]*Result{testResult}, nil)
			},
			expectedResults: []*Result{testResult},
		},
		{
			name:    "returns error from validation",
			reqType: QueryTypePreparedQuery,
			configureDataFetcher: func(t *testing.T, fetcher *MockCatalogDataFetcher) {
				fetcher.On("ValidateRequest", mock.Anything, mock.Anything).Return(testErr)
			},
			expectedError: testErr,
		},
		{
			name:    "returns error from fetcher",
			reqType: QueryTypePreparedQuery,
			configureDataFetcher: func(t *testing.T, fetcher *MockCatalogDataFetcher) {
				fetcher.On("ValidateRequest", mock.Anything, mock.Anything).Return(nil)
				fetcher.On("NormalizeRequest", mock.Anything)
				fetcher.On("FetchPreparedQuery", mock.Anything, mock.Anything).Return(nil, testErr)
			},
			expectedError: testErr,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestQueryByIP(t *testing.T) {
	type testCase struct {
		name                 string
		configureDataFetcher func(*testing.T, *MockCatalogDataFetcher)
		expectedResults      []*Result
		expectedError        error
	}

	run := func(t *testing.T, tc testCase) {

		fetcher := NewMockCatalogDataFetcher(t)
		tc.configureDataFetcher(t, fetcher)

		qp := NewQueryProcessor(fetcher)

		results, err := qp.QueryByIP(testIP, testContext)
		if tc.expectedError != nil {
			require.Error(t, err)
			require.True(t, errors.Is(err, tc.expectedError))
			require.Nil(t, results)
			return
		}
		require.NoError(t, err)
		require.Equal(t, tc.expectedResults, results)
	}

	testCases := []testCase{
		{
			name: "query by IP",
			configureDataFetcher: func(t *testing.T, fetcher *MockCatalogDataFetcher) {
				fetcher.On("FetchRecordsByIp", mock.Anything, mock.Anything).Return([]*Result{testResult}, nil)
			},
			expectedResults: []*Result{testResult},
		},
		{
			name: "returns error from fetcher",
			configureDataFetcher: func(t *testing.T, fetcher *MockCatalogDataFetcher) {
				fetcher.On("FetchRecordsByIp", mock.Anything, mock.Anything).Return(nil, testErr)
			},
			expectedError: testErr,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}
