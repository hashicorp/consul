// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package demo includes fake resource types for working on Consul's generic
// state storage without having to refer to specific features.
package demo

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
	pbdemov1 "github.com/hashicorp/consul/proto/private/pbdemo/v1"
	pbdemov2 "github.com/hashicorp/consul/proto/private/pbdemo/v2"
)

var (
	// TypeV1Executive represents a a C-suite executive of the company.
	// Used as a resource to test cluster scope.
	TypeV1Executive = &pbresource.Type{
		Group:        "demo",
		GroupVersion: "v1",
		Kind:         "Executive",
	}

	// TypeV1RecordLabel represents a record label which artists are signed to.
	// Used as a resource to test partiion scope.
	TypeV1RecordLabel = &pbresource.Type{
		Group:        "demo",
		GroupVersion: "v1",
		Kind:         "RecordLabel",
	}

	// TypeV1Artist represents a musician or group of musicians.
	TypeV1Artist = &pbresource.Type{
		Group:        "demo",
		GroupVersion: "v1",
		Kind:         "Artist",
	}

	// TypeV1Album represents a collection of an artist's songs.
	TypeV1Album = &pbresource.Type{
		Group:        "demo",
		GroupVersion: "v1",
		Kind:         "Album",
	}

	// TypeV1Concept represents an abstract concept that can be associated with any other resource.
	TypeV1Concept = &pbresource.Type{
		Group:        "demo",
		GroupVersion: "v1",
		Kind:         "Concept",
	}

	// TypeV2Artist represents a musician or group of musicians.
	TypeV2Artist = &pbresource.Type{
		Group:        "demo",
		GroupVersion: "v2",
		Kind:         "Artist",
	}

	// TypeV2Album represents a collection of an artist's songs.
	TypeV2Album = &pbresource.Type{
		Group:        "demo",
		GroupVersion: "v2",
		Kind:         "Album",
	}

	// TypeV2Festival represents a named collection of artists and genres.
	TypeV2Festival = &pbresource.Type{
		Group:        "demo",
		GroupVersion: "v2",
		Kind:         "Festival",
	}
)

const (
	ArtistV1ReadPolicy  = `key_prefix "resource/demo.v1.Artist/" { policy = "read" }`
	ArtistV1WritePolicy = `key_prefix "resource/demo.v1.Artist/" { policy = "write" }`
	ArtistV2ReadPolicy  = `key_prefix "resource/demo.v2.Artist/" { policy = "read" }`
	ArtistV2WritePolicy = `key_prefix "resource/demo.v2.Artist/" { policy = "write" }`
	ArtistV2ListPolicy  = `key_prefix "resource/" { policy = "list" }`

	ExecutiveV1ReadPolicy  = `key_prefix "resource/demo.v1.Executive/" { policy = "read" }`
	ExecutiveV1WritePolicy = `key_prefix "resource/demo.v1.Executive/" { policy = "write" }`

	LabelV1ReadPolicy  = `key_prefix "resource/demo.v1.Label/" { policy = "read" }`
	LabelV1WritePolicy = `key_prefix "resource/demo.v1.Label/" { policy = "write" }`
)

// RegisterTypes registers the demo types. Should only be called in tests and
// dev mode.
//
// TODO(spatel): We're standing-in key ACLs for demo resources until our ACL
// system can be more modularly extended (or support generic resource permissions).
func RegisterTypes(r resource.Registry) {
	readACL := func(authz acl.Authorizer, authzContext *acl.AuthorizerContext, id *pbresource.ID, res *pbresource.Resource) error {
		if resource.EqualType(TypeV1RecordLabel, id.Type) {
			if res == nil {
				return resource.ErrNeedResource
			}
		}
		key := fmt.Sprintf("resource/%s/%s", resource.ToGVK(id.Type), id.Name)
		return authz.ToAllowAuthorizer().KeyReadAllowed(key, authzContext)
	}

	writeACL := func(authz acl.Authorizer, authzContext *acl.AuthorizerContext, res *pbresource.Resource) error {
		key := fmt.Sprintf("resource/%s/%s", resource.ToGVK(res.Id.Type), res.Id.Name)
		return authz.ToAllowAuthorizer().KeyWriteAllowed(key, authzContext)
	}

	makeListACL := func(typ *pbresource.Type) func(acl.Authorizer, *acl.AuthorizerContext) error {
		return func(authz acl.Authorizer, authzContext *acl.AuthorizerContext) error {
			key := fmt.Sprintf("resource/%s", resource.ToGVK(typ))
			return authz.ToAllowAuthorizer().KeyListAllowed(key, authzContext)
		}
	}

	validateV1ArtistFn := func(res *pbresource.Resource) error {
		artist := &pbdemov1.Artist{}
		if err := anypb.UnmarshalTo(res.Data, artist, proto.UnmarshalOptions{}); err != nil {
			return err
		}
		if artist.Name == "" {
			return fmt.Errorf("artist.name required")
		}
		return nil
	}

	validateV2ArtistFn := func(res *pbresource.Resource) error {
		artist := &pbdemov2.Artist{}
		if err := anypb.UnmarshalTo(res.Data, artist, proto.UnmarshalOptions{}); err != nil {
			return err
		}
		if artist.Name == "" {
			return fmt.Errorf("artist.name required")
		}
		return nil
	}

	mutateV2ArtistFn := func(res *pbresource.Resource) error {
		// Not a realistic use for this hook, but set genre if not specified
		artist := &pbdemov2.Artist{}
		if err := anypb.UnmarshalTo(res.Data, artist, proto.UnmarshalOptions{}); err != nil {
			return err
		}
		if artist.Genre == pbdemov2.Genre_GENRE_UNSPECIFIED {
			artist.Genre = pbdemov2.Genre_GENRE_DISCO
			return res.Data.MarshalFrom(artist)
		}
		return nil
	}

	r.Register(resource.Registration{
		Type:  TypeV1Executive,
		Proto: &pbdemov1.Executive{},
		Scope: resource.ScopeCluster,
		ACLs: &resource.ACLHooks{
			Read:  readACL,
			Write: writeACL,
			List:  makeListACL(TypeV1Executive),
		},
	})

	r.Register(resource.Registration{
		Type:  TypeV1RecordLabel,
		Proto: &pbdemov1.RecordLabel{},
		Scope: resource.ScopePartition,
		ACLs: &resource.ACLHooks{
			Read:  readACL,
			Write: writeACL,
			List:  makeListACL(TypeV1RecordLabel),
		},
	})

	r.Register(resource.Registration{
		Type:  TypeV1Artist,
		Proto: &pbdemov1.Artist{},
		Scope: resource.ScopeNamespace,
		ACLs: &resource.ACLHooks{
			Read:  readACL,
			Write: writeACL,
			List:  makeListACL(TypeV1Artist),
		},
		Validate: validateV1ArtistFn,
	})

	r.Register(resource.Registration{
		Type:  TypeV1Album,
		Proto: &pbdemov1.Album{},
		Scope: resource.ScopeNamespace,
		ACLs: &resource.ACLHooks{
			Read:  readACL,
			Write: writeACL,
			List:  makeListACL(TypeV1Album),
		},
	})

	r.Register(resource.Registration{
		Type:  TypeV1Concept,
		Proto: &pbdemov1.Concept{},
		Scope: resource.ScopeNamespace,
		ACLs: &resource.ACLHooks{
			Read:  readACL,
			Write: writeACL,
			List:  makeListACL(TypeV1Concept),
		},
	})

	r.Register(resource.Registration{
		Type:  TypeV2Artist,
		Proto: &pbdemov2.Artist{},
		Scope: resource.ScopeNamespace,
		ACLs: &resource.ACLHooks{
			Read:  readACL,
			Write: writeACL,
			List:  makeListACL(TypeV2Artist),
		},
		Validate: validateV2ArtistFn,
		Mutate:   mutateV2ArtistFn,
	})

	r.Register(resource.Registration{
		Type:  TypeV2Album,
		Proto: &pbdemov2.Album{},
		Scope: resource.ScopeNamespace,
		ACLs: &resource.ACLHooks{
			Read:  readACL,
			Write: writeACL,
			List:  makeListACL(TypeV2Album),
		},
	})

	r.Register(resource.Registration{
		Type:  TypeV2Festival,
		Proto: &pbdemov2.Festival{},
		Scope: resource.ScopeNamespace,
		ACLs: &resource.ACLHooks{
			Read:  readACL,
			Write: writeACL,
			List:  makeListACL(TypeV2Festival),
		},
	})
}

// GenerateV1Executive generates a named Executive resource.
func GenerateV1Executive(name, position string) (*pbresource.Resource, error) {
	data, err := anypb.New(&pbdemov1.Executive{Position: position})
	if err != nil {
		return nil, err
	}

	return &pbresource.Resource{
		Id: &pbresource.ID{
			Type:    TypeV1Executive,
			Tenancy: resource.DefaultClusteredTenancy(),
			Name:    name,
		},
		Data: data,
		Metadata: map[string]string{
			"generated_at": time.Now().Format(time.RFC3339),
		},
	}, nil
}

// GenerateV1RecordLabel generates a named RecordLabel resource.
func GenerateV1RecordLabel(name string) (*pbresource.Resource, error) {
	data, err := anypb.New(&pbdemov1.RecordLabel{Name: name})
	if err != nil {
		return nil, err
	}

	return &pbresource.Resource{
		Id: &pbresource.ID{
			Type:    TypeV1RecordLabel,
			Tenancy: resource.DefaultPartitionedTenancy(),
			Name:    name,
		},
		Data: data,
		Metadata: map[string]string{
			"generated_at": time.Now().Format(time.RFC3339),
		},
	}, nil
}

// GenerateV1Concept generates a named concept resource.
func GenerateV1Concept(name string) (*pbresource.Resource, error) {
	return &pbresource.Resource{
		Id: &pbresource.ID{
			Type:    TypeV1Concept,
			Tenancy: resource.DefaultPartitionedTenancy(),
			Name:    name,
		},
		Data: nil,
		Metadata: map[string]string{
			"generated_at": time.Now().Format(time.RFC3339),
		},
	}, nil
}

// GenerateV2Artist generates a random Artist resource.
func GenerateV2Artist() (*pbresource.Resource, error) {
	adjective := adjectives[rand.Intn(len(adjectives))]
	noun := nouns[rand.Intn(len(nouns))]

	numMembers := rand.Intn(5) + 1
	groupMembers := make(map[string]string, numMembers)
	for i := 0; i < numMembers; i++ {
		groupMembers[members[rand.Intn(len(members))]] = instruments[rand.Intn(len(instruments))]
	}

	data, err := anypb.New(&pbdemov2.Artist{
		Name:         fmt.Sprintf("%s %s", adjective, noun),
		Genre:        randomGenre(),
		GroupMembers: groupMembers,
	})
	if err != nil {
		return nil, err
	}

	return &pbresource.Resource{
		Id: &pbresource.ID{
			Type:    TypeV2Artist,
			Tenancy: resource.DefaultNamespacedTenancy(),
			Name:    fmt.Sprintf("%s-%s", strings.ToLower(adjective), strings.ToLower(noun)),
		},
		Data: data,
		Metadata: map[string]string{
			"generated_at": time.Now().Format(time.RFC3339),
		},
	}, nil
}

// GenerateV2Album generates a random Album resource, owned by the Artist with
// the given ID.
func GenerateV2Album(artistID *pbresource.ID) (*pbresource.Resource, error) {
	return generateV2Album(artistID, rand.New(rand.NewSource(time.Now().UnixNano())))
}

func generateV2Album(artistID *pbresource.ID, rand *rand.Rand) (*pbresource.Resource, error) {
	adjective := adjectives[rand.Intn(len(adjectives))]
	noun := nouns[rand.Intn(len(nouns))]

	numTracks := 3 + rand.Intn(3)
	tracks := make([]string, numTracks)
	for i := 0; i < numTracks; i++ {
		words := nouns
		if i%3 == 0 {
			words = adjectives
		}
		tracks[i] = words[rand.Intn(len(words))]
	}

	data, err := anypb.New(&pbdemov2.Album{
		Title:              fmt.Sprintf("%s %s", adjective, noun),
		YearOfRelease:      int32(1990 + rand.Intn(time.Now().Year()-1990)),
		CriticallyAclaimed: rand.Int()%2 == 0,
		Tracks:             tracks,
	})
	if err != nil {
		return nil, err
	}

	return &pbresource.Resource{
		Id: &pbresource.ID{
			Type:    TypeV2Album,
			Tenancy: clone(artistID.Tenancy),
			Name:    fmt.Sprintf("%s-%s-%s", artistID.Name, strings.ToLower(adjective), strings.ToLower(noun)),
		},
		Owner: artistID,
		Data:  data,
		Metadata: map[string]string{
			"generated_at": time.Now().Format(time.RFC3339),
		},
	}, nil
}

func randomGenre() pbdemov2.Genre {
	return pbdemov2.Genre(rand.Intn(len(pbdemov1.Genre_name)-1) + 1)
}

var (
	adjectives = []string{
		"Purple",
		"Angry",
		"Euphoric",
		"Unexpected",
		"Cheesy",
		"Rancid",
		"Pleasant",
		"Mumbling",
		"Enlightened",
	}

	nouns = []string{
		"Speakerphone",
		"Fox",
		"Guppy",
		"Smile",
		"Emacs",
		"Grapefruit",
		"Engineer",
		"Basketball",
	}

	members = []string{
		"Owl",
		"Tiger",
		"Beetle",
		"Lion",
		"Chicken",
		"Snake",
		"Monkey",
		"Kitten",
		"Hound",
	}

	instruments = []string{
		"Guitar",
		"Bass",
		"Lead Vocals",
		"Backing Vocals",
		"Drums",
		"Synthesizer",
		"Triangle",
		"Standing by the stage looking cool",
	}
)

func clone[T proto.Message](v T) T { return proto.Clone(v).(T) }
