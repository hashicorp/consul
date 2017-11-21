package token

import (
	"testing"
)

func TestStore_RegularTokens(t *testing.T) {
	t.Parallel()

	type tokens struct {
		user, agent, repl string
	}

	tests := []struct {
		name      string
		set, want tokens
	}{
		{
			name: "set user",
			set:  tokens{user: "U"},
			want: tokens{user: "U", agent: "U"},
		},
		{
			name: "set agent",
			set:  tokens{agent: "A"},
			want: tokens{agent: "A"},
		},
		{
			name: "set user and agent",
			set:  tokens{agent: "A", user: "U"},
			want: tokens{agent: "A", user: "U"},
		},
		{
			name: "set repl",
			set:  tokens{repl: "R"},
			want: tokens{repl: "R"},
		},
		{
			name: "set all",
			set:  tokens{user: "U", agent: "A", repl: "R"},
			want: tokens{user: "U", agent: "A", repl: "R"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := new(Store)
			s.UpdateUserToken(tt.set.user)
			s.UpdateAgentToken(tt.set.agent)
			s.UpdateACLReplicationToken(tt.set.repl)
			if got, want := s.UserToken(), tt.want.user; got != want {
				t.Fatalf("got token %q want %q", got, want)
			}
			if got, want := s.AgentToken(), tt.want.agent; got != want {
				t.Fatalf("got token %q want %q", got, want)
			}
			if got, want := s.ACLReplicationToken(), tt.want.repl; got != want {
				t.Fatalf("got token %q want %q", got, want)
			}
		})
	}
}

func TestStore_AgentMasterToken(t *testing.T) {
	t.Parallel()
	s := new(Store)

	verify := func(want bool, toks ...string) {
		for _, tok := range toks {
			if got := s.IsAgentMasterToken(tok); got != want {
				t.Fatalf("token %q got %v want %v", tok, got, want)
			}
		}
	}

	verify(false, "", "nope")

	s.UpdateAgentMasterToken("master")
	verify(true, "master")
	verify(false, "", "nope")

	s.UpdateAgentMasterToken("another")
	verify(true, "another")
	verify(false, "", "nope", "master")

	s.UpdateAgentMasterToken("")
	verify(false, "", "nope", "master", "another")
}
