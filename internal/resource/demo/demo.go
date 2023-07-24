// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

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
	// TenancyDefault contains the default values for all tenancy units.
	TenancyDefault = &pbresource.Tenancy{
		Partition: "default",
		PeerName:  "local",
		Namespace: "default",
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
)

const (
	ArtistV1ReadPolicy  = `key_prefix "resource/demo.v1.Artist/" { policy = "read" }`
	ArtistV1WritePolicy = `key_prefix "resource/demo.v1.Artist/" { policy = "write" }`
	ArtistV2ReadPolicy  = `key_prefix "resource/demo.v2.Artist/" { policy = "read" }`
	ArtistV2WritePolicy = `key_prefix "resource/demo.v2.Artist/" { policy = "write" }`
	ArtistV2ListPolicy  = `key_prefix "resource/" { policy = "list" }`
)

// RegisterTypes registers the demo types. Should only be called in tests and
// dev mode.
//
// TODO(spatel): We're standing-in key ACLs for demo resources until our ACL
// system can be more modularly extended (or support generic resource permissions).
func RegisterTypes(r resource.Registry) {
	readACL := func(authz acl.Authorizer, id *pbresource.ID) error {
		key := fmt.Sprintf("resource/%s/%s", resource.ToGVK(id.Type), id.Name)
		return authz.ToAllowAuthorizer().KeyReadAllowed(key, &acl.AuthorizerContext{})
	}

	writeACL := func(authz acl.Authorizer, res *pbresource.Resource) error {
		key := fmt.Sprintf("resource/%s/%s", resource.ToGVK(res.Id.Type), res.Id.Name)
		return authz.ToAllowAuthorizer().KeyWriteAllowed(key, &acl.AuthorizerContext{})
	}

	makeListACL := func(typ *pbresource.Type) func(acl.Authorizer, *pbresource.Tenancy) error {
		return func(authz acl.Authorizer, tenancy *pbresource.Tenancy) error {
			key := fmt.Sprintf("resource/%s", resource.ToGVK(typ))
			return authz.ToAllowAuthorizer().KeyListAllowed(key, &acl.AuthorizerContext{})
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
		Type:  TypeV1Artist,
		Proto: &pbdemov1.Artist{},
		ACLs: &resource.ACLHooks{
			Read:  readACL,
			Write: writeACL,
			List:  makeListACL(TypeV1Artist),
		},
		Validate: validateV1ArtistFn,
		Scope:    resource.ScopeNamespace,
	})

	r.Register(resource.Registration{
		Type:  TypeV1Album,
		Proto: &pbdemov1.Album{},
		ACLs: &resource.ACLHooks{
			Read:  readACL,
			Write: writeACL,
			List:  makeListACL(TypeV1Album),
		},
		Scope: resource.ScopeNamespace,
	})

	r.Register(resource.Registration{
		Type:  TypeV2Artist,
		Proto: &pbdemov2.Artist{},
		ACLs: &resource.ACLHooks{
			Read:  readACL,
			Write: writeACL,
			List:  makeListACL(TypeV2Artist),
		},
		Validate: validateV2ArtistFn,
		Mutate:   mutateV2ArtistFn,
		Scope:    resource.ScopeNamespace,
	})

	r.Register(resource.Registration{
		Type:  TypeV2Album,
		Proto: &pbdemov2.Album{},
		ACLs: &resource.ACLHooks{
			Read:  readACL,
			Write: writeACL,
			List:  makeListACL(TypeV2Album),
		},
		Scope: resource.ScopeNamespace,
	})
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
			Tenancy: TenancyDefault,
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
			Tenancy: artistID.Tenancy,
			Name:    fmt.Sprintf("%s/%s-%s", artistID.Name, strings.ToLower(adjective), strings.ToLower(noun)),
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
