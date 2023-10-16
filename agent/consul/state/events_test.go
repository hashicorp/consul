// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/proto/private/pbsubscribe"
)

const aclToken = "67b04fbc-e35f-494a-ad43-739f1c8b839c"

func TestPBToStreamSubscribeRequest(t *testing.T) {
	cases := map[string]struct {
		req                      *pbsubscribe.SubscribeRequest
		entMeta                  acl.EnterpriseMeta
		expectedSubscribeRequest *stream.SubscribeRequest
		err                      error
	}{
		"Wildcard subject": {
			req: &pbsubscribe.SubscribeRequest{
				Topic:   EventTopicServiceList,
				Subject: &pbsubscribe.SubscribeRequest_WildcardSubject{WildcardSubject: true},
				Token:   aclToken,
				Index:   1,
			},
			entMeta: acl.EnterpriseMeta{},
			expectedSubscribeRequest: &stream.SubscribeRequest{
				Topic:   EventTopicServiceList,
				Subject: stream.SubjectWildcard,
				Token:   aclToken,
				Index:   1,
			},
			err: nil,
		},
		"Deprecated top level fields": {
			req: &pbsubscribe.SubscribeRequest{
				Topic:     EventTopicServiceDefaults,
				Key:       "key",
				Partition: "partition",
				Namespace: "consul",
				PeerName:  "peer",
			},
			entMeta: acl.EnterpriseMeta{},
			expectedSubscribeRequest: &stream.SubscribeRequest{
				Topic: EventTopicServiceDefaults,
				Subject: EventSubjectConfigEntry{
					Name:           "key",
					EnterpriseMeta: &acl.EnterpriseMeta{},
				},
			},
			err: nil,
		},
		"Service health": {
			req: &pbsubscribe.SubscribeRequest{
				Topic: EventTopicServiceHealth,
				Subject: &pbsubscribe.SubscribeRequest_NamedSubject{
					NamedSubject: &pbsubscribe.NamedSubject{
						Key:       "key",
						Namespace: "consul",
						Partition: "partition",
						PeerName:  "peer",
					},
				},
				Token: aclToken,
				Index: 2,
			},
			entMeta: acl.EnterpriseMeta{},
			expectedSubscribeRequest: &stream.SubscribeRequest{
				Topic: EventTopicServiceHealth,
				Subject: EventSubjectService{
					Key:            "key",
					EnterpriseMeta: acl.EnterpriseMeta{},
					PeerName:       "peer",
				},
				Token: aclToken,
				Index: 2,
			},
			err: nil,
		},
		"Sameness Group": {
			req: &pbsubscribe.SubscribeRequest{
				Topic: EventTopicSamenessGroup,
				Subject: &pbsubscribe.SubscribeRequest_NamedSubject{
					NamedSubject: &pbsubscribe.NamedSubject{
						Key:       "sg",
						Namespace: "consul",
						Partition: "partition",
						PeerName:  "peer",
					},
				},
				Token: aclToken,
				Index: 2,
			},
			entMeta: acl.EnterpriseMeta{},
			expectedSubscribeRequest: &stream.SubscribeRequest{
				Topic: EventTopicSamenessGroup,
				Subject: EventSubjectConfigEntry{
					Name:           "sg",
					EnterpriseMeta: &acl.EnterpriseMeta{},
				},
				Token: aclToken,
				Index: 2,
			},
			err: nil,
		},
		"Config": {
			req: &pbsubscribe.SubscribeRequest{
				Topic: EventTopicAPIGateway,
				Subject: &pbsubscribe.SubscribeRequest_NamedSubject{
					NamedSubject: &pbsubscribe.NamedSubject{
						Key:       "key",
						Namespace: "consul",
						Partition: "partition",
						PeerName:  "peer",
					},
				},
				Token: aclToken,
				Index: 2,
			},
			entMeta: acl.EnterpriseMeta{},
			expectedSubscribeRequest: &stream.SubscribeRequest{
				Topic: EventTopicAPIGateway,
				Subject: EventSubjectConfigEntry{
					Name:           "key",
					EnterpriseMeta: &acl.EnterpriseMeta{},
				},
				Token: aclToken,
				Index: 2,
			},
			err: nil,
		},
		"Service list without wildcard returns error": {
			req: &pbsubscribe.SubscribeRequest{
				Topic: EventTopicServiceList,
				Subject: &pbsubscribe.SubscribeRequest_NamedSubject{
					NamedSubject: &pbsubscribe.NamedSubject{
						Key:       "key",
						Namespace: "consul",
						Partition: "partition",
						PeerName:  "peer",
					},
				},
			},
			entMeta:                  acl.EnterpriseMeta{},
			expectedSubscribeRequest: nil,
			err:                      fmt.Errorf("topic %s can only be consumed using WildcardSubject", EventTopicServiceList),
		},
		"Unrecognized topic returns error": {
			req: &pbsubscribe.SubscribeRequest{
				Topic: 99999,
				Subject: &pbsubscribe.SubscribeRequest_NamedSubject{
					NamedSubject: &pbsubscribe.NamedSubject{
						Key:       "key",
						Namespace: "consul",
						Partition: "partition",
						PeerName:  "peer",
					},
				},
			},
			entMeta:                  acl.EnterpriseMeta{},
			expectedSubscribeRequest: nil,
			err:                      fmt.Errorf("cannot construct subject for topic 99999"),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			actual, err := PBToStreamSubscribeRequest(tc.req, tc.entMeta)

			if tc.err != nil {
				require.EqualError(t, err, tc.err.Error())
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, tc.expectedSubscribeRequest, actual)
		})
	}
}
