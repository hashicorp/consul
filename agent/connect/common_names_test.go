package connect

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServiceAndNamespaceTruncation(t *testing.T) {
	type tcase struct {
		service   string
		namespace string
		// if left as "" then its expected to not be truncated
		expectedService string
		// if left as "" then its expected to not be truncated
		expectedNamespace string
	}

	cases := map[string]tcase{
		"short-no-truncation": {
			service:   "foo",
			namespace: "bar",
		},
		"long-service-no-truncation": {
			// -3 because thats the length of the namespace
			service:   strings.Repeat("a", maxServiceAndNamespaceLen-3),
			namespace: "bar",
		},
		"long-namespace-no-truncation": {
			service: "foo",
			// -3 because thats the length of the service name
			namespace: strings.Repeat("b", maxServiceAndNamespaceLen-3),
		},
		"truncate-service-only": {
			// this should force the service name to be truncated
			service:         strings.Repeat("a", maxServiceAndNamespaceLen-minNamespaceNameLen+5),
			expectedService: strings.Repeat("a", maxServiceAndNamespaceLen-minNamespaceNameLen),
			// this is the maximum length that will never be truncated for a namespace
			namespace: strings.Repeat("b", minNamespaceNameLen),
		},
		"truncate-namespace-only": {
			// this is the maximum length that will never be truncated for a service name
			service: strings.Repeat("a", minServiceNameLen),
			// this should force the namespace name to be truncated
			namespace:         strings.Repeat("b", maxServiceAndNamespaceLen-minServiceNameLen+5),
			expectedNamespace: strings.Repeat("b", maxServiceAndNamespaceLen-minServiceNameLen),
		},
		"truncate-both-even": {
			// this test would need to be update if the maxServiceAndNamespaceLen variable is updated
			// I could put some more complex logic into here to prevent that but it would be mostly
			// duplicating the logic in the function itself and thus not really be testing anything
			//
			// The original lengths of 50 / 51 were chose when maxServiceAndNamespaceLen would be 43
			// Therefore a total of 58 characters must be removed. This was chose so that the value
			// could be evenly split between the two strings.
			service:           strings.Repeat("a", 50),
			expectedService:   strings.Repeat("a", 21),
			namespace:         strings.Repeat("b", 51),
			expectedNamespace: strings.Repeat("b", 22),
		},
		"truncate-both-odd": {
			// this test would need to be update if the maxServiceAndNamespaceLen variable is updated
			// I could put some more complex logic into here to prevent that but it would be mostly
			// duplicating the logic in the function itself and thus not really be testing anything
			//
			// The original lengths of 50 / 57 were chose when maxServiceAndNamespaceLen would be 43
			// Therefore a total of 57 characters must be removed. This was chose so that the value
			// could not be evenly removed from both so the namespace should be truncated to a length
			// 1 character more than the service.
			service:           strings.Repeat("a", 50),
			expectedService:   strings.Repeat("a", 21),
			namespace:         strings.Repeat("b", 50),
			expectedNamespace: strings.Repeat("b", 22),
		},
		"truncate-both-min-svc": {
			service:           strings.Repeat("a", minServiceNameLen+1),
			expectedService:   strings.Repeat("a", minServiceNameLen),
			namespace:         strings.Repeat("b", maxServiceAndNamespaceLen),
			expectedNamespace: strings.Repeat("b", maxServiceAndNamespaceLen-minServiceNameLen),
		},
		"truncate-both-min-ns": {
			service:           strings.Repeat("a", maxServiceAndNamespaceLen),
			expectedService:   strings.Repeat("a", maxServiceAndNamespaceLen-minNamespaceNameLen),
			namespace:         strings.Repeat("b", minNamespaceNameLen+1),
			expectedNamespace: strings.Repeat("b", minNamespaceNameLen),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			actualSvc, actualNamespace := truncateServiceAndNamespace(tc.service, tc.namespace)

			expectedLen := len(tc.service) + len(tc.namespace)
			if tc.expectedService != "" || tc.expectedNamespace != "" {
				expectedLen = maxServiceAndNamespaceLen
			}

			actualLen := len(actualSvc) + len(actualNamespace)

			require.Equal(t, expectedLen, actualLen, "Combined length of %d (svc: %d, ns: %d) does not match expected value of %d", actualLen, len(actualSvc), len(actualNamespace), expectedLen)

			if tc.expectedService != "" {
				require.Equal(t, tc.expectedService, actualSvc)
			} else {
				require.Equal(t, tc.service, actualSvc)
			}
			if tc.expectedNamespace != "" {
				require.Equal(t, tc.expectedNamespace, actualNamespace)
			} else {
				require.Equal(t, tc.namespace, actualNamespace)
			}
		})
	}
}
