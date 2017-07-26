package token

import (
	"testing"
)

func TestStore_UserAndAgentTokens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		user, agent, wantUser, wantAgent string
	}{
		{"", "", "", ""},
		{"user", "", "user", "user"},
		{"user", "agent", "user", "agent"},
		{"", "agent", "", "agent"},
		{"user", "agent", "user", "agent"},
		{"user", "", "user", "user"},
		{"", "", "", ""},
	}
	tokens := new(Store)
	for _, tt := range tests {
		tokens.UpdateUserToken(tt.user)
		tokens.UpdateAgentToken(tt.agent)
		if got, want := tokens.UserToken(), tt.wantUser; got != want {
			t.Fatalf("got token %q want %q", got, want)
		}
		if got, want := tokens.AgentToken(), tt.wantAgent; got != want {
			t.Fatalf("got token %q want %q", got, want)
		}
	}
}

func TestStore_AgentMasterToken(t *testing.T) {
	t.Parallel()
	tokens := new(Store)

	verify := func(want bool, toks ...string) {
		for _, tok := range toks {
			if got := tokens.IsAgentMasterToken(tok); got != want {
				t.Fatalf("token %q got %v want %v", tok, got, want)
			}
		}
	}

	verify(false, "", "nope")

	tokens.UpdateAgentMasterToken("master")
	verify(true, "master")
	verify(false, "", "nope")

	tokens.UpdateAgentMasterToken("another")
	verify(true, "another")
	verify(false, "", "nope", "master")

	tokens.UpdateAgentMasterToken("")
	verify(false, "", "nope", "master", "another")
}
