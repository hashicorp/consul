package state

import (
	"errors"
	"fmt"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/proto/pbsubscribe"
)

func PBToStreamSubscribeRequest(req *pbsubscribe.SubscribeRequest, entMeta acl.EnterpriseMeta) (*stream.SubscribeRequest, error) {
	var subject stream.Subject

	if req.GetWildcardSubject() {
		subject = stream.SubjectWildcard
	} else {
		named := req.GetNamedSubject()

		// Support the (deprcated) top-level Key, Partition, Namespace, and PeerName fields.
		if named == nil {
			named = &pbsubscribe.NamedSubject{
				Key:       req.Key,       // nolint:staticcheck // SA1019 intentional use of deprecated field
				Partition: req.Partition, // nolint:staticcheck // SA1019 intentional use of deprecated field
				Namespace: req.Namespace, // nolint:staticcheck // SA1019 intentional use of deprecated field
				PeerName:  req.PeerName,  // nolint:staticcheck // SA1019 intentional use of deprecated field
			}
		}

		if named.Key == "" {
			return nil, errors.New("either WildcardSubject or NamedSubject.Key is required")
		}

		switch req.Topic {
		case EventTopicServiceHealth, EventTopicServiceHealthConnect:
			subject = EventSubjectService{
				Key:            named.Key,
				EnterpriseMeta: entMeta,
				PeerName:       named.PeerName,
			}
		case EventTopicMeshConfig, EventTopicServiceResolver, EventTopicIngressGateway:
			subject = EventSubjectConfigEntry{
				Name:           named.Key,
				EnterpriseMeta: &entMeta,
			}
		default:
			return nil, fmt.Errorf("cannot construct subject for topic %s", req.Topic)
		}
	}

	return &stream.SubscribeRequest{
		Topic:   req.Topic,
		Subject: subject,
		Token:   req.Token,
		Index:   req.Index,
	}, nil
}
