package demo

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
	pbdemov1 "github.com/hashicorp/consul/proto/private/pbdemo/v1"
)

var TenancyDefault = &pbresource.Tenancy{
	Partition: "default",
	PeerName:  "local",
	Namespace: "default",
}

var TypeArtist = &pbresource.Type{
	Group:        "demo",
	GroupVersion: "v1",
	Kind:         "artist",
}

var TypeAlbum = &pbresource.Type{
	Group:        "demo",
	GroupVersion: "v1",
	Kind:         "album",
}

func Register(r resource.Registry) {
	r.Register(resource.Registration{
		Type: TypeArtist,
	})

	r.Register(resource.Registration{
		Type: TypeAlbum,
	})
}

func GenerateArtist() (*pbresource.Resource, error) {
	adjective := adjectives[rand.Intn(len(adjectives))]
	noun := nouns[rand.Intn(len(nouns))]

	data, err := anypb.New(&pbdemov1.Artist{
		Name:         fmt.Sprintf("%s %s", adjective, noun),
		Genre:        randomGenre(),
		GroupMembers: int32(rand.Intn(5) + 1),
	})
	if err != nil {
		return nil, err
	}

	return &pbresource.Resource{
		Id: &pbresource.ID{
			Type:    TypeArtist,
			Tenancy: TenancyDefault,
			Name:    fmt.Sprintf("%s-%s", strings.ToLower(adjective), strings.ToLower(noun)),
		},
		Data: data,
		Metadata: map[string]string{
			"generated_at": time.Now().Format(time.RFC3339),
		},
	}, nil
}

func GenerateAlbum(artistID *pbresource.ID) (*pbresource.Resource, error) {
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

	data, err := anypb.New(&pbdemov1.Album{
		Name:               fmt.Sprintf("%s %s", adjective, noun),
		YearOfRelease:      int32(1990 + rand.Intn(time.Now().Year()-1990)),
		CriticallyAclaimed: rand.Int()%2 == 0,
		Tracks:             tracks,
	})
	if err != nil {
		return nil, err
	}

	return &pbresource.Resource{
		Id: &pbresource.ID{
			Type:    TypeAlbum,
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

func randomGenre() pbdemov1.Genre {
	return pbdemov1.Genre(rand.Intn(len(pbdemov1.Genre_name)-1) + 1)
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
)
