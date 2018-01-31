package config

import (
	"testing"
	"time"

	"github.com/pascaldekloe/goe/verify"
)

func TestMerge(t *testing.T) {
	tests := []struct {
		desc string
		cfgs []Config
		want Config
	}{
		{
			"top level fields",
			[]Config{
				{AdvertiseAddrLAN: pString("a")},
				{AdvertiseAddrLAN: pString("b")},
				{RaftProtocol: pInt(1)},
				{RaftProtocol: pInt(2)},
				{ServerMode: pBool(false)},
				{ServerMode: pBool(true)},
				{StartJoinAddrsLAN: []string{"a"}},
				{StartJoinAddrsLAN: []string{"b"}},
				{NodeMeta: map[string]string{"a": "b"}},
				{NodeMeta: map[string]string{"c": "d"}},
				{NodeMeta: map[string]string{"c": "e"}},
				{Ports: Ports{DNS: pInt(1)}},
				{Ports: Ports{DNS: pInt(2), HTTP: pInt(3)}},
			},
			Config{
				AdvertiseAddrLAN:  pString("b"),
				RaftProtocol:      pInt(2),
				ServerMode:        pBool(true),
				StartJoinAddrsLAN: []string{"a", "b"},
				NodeMeta: map[string]string{
					"a": "b",
					"c": "e",
				},
				Ports: Ports{DNS: pInt(2), HTTP: pInt(3)},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got, want := Merge(tt.cfgs...), tt.want
			if !verify.Values(t, "", got, want) {
				t.FailNow()
			}
		})
	}
}

func pBool(v bool) *bool                { return &v }
func pInt(v int) *int                   { return &v }
func pString(v string) *string          { return &v }
func pDuration(v time.Duration) *string { s := v.String(); return &s }
func pFloat64(v float64) *float64       { return &v }
