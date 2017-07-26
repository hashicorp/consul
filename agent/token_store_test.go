package agent

import (
	"testing"
)

func TestTokenStore_AgentToken(t *testing.T) {
	t.Parallel()
	tokens := new(TokenStore)

	if got, want := tokens.AgentToken(), ""; got != want {
		t.Fatalf("got %q want %q", got, want)
	}

	// Precedence is agent token, user token, "".

	tokens.UpdateUserToken("user")
	if got, want := tokens.AgentToken(), "user"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}

	tokens.UpdateAgentToken("agent")
	if got, want := tokens.AgentToken(), "agent"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}

	tokens.UpdateUserToken("")
	if got, want := tokens.AgentToken(), "agent"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}

	tokens.UpdateAgentToken("")
	if got, want := tokens.AgentToken(), ""; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestTokenStore_UserToken(t *testing.T) {
	t.Parallel()
	tokens := new(TokenStore)

	if got, want := tokens.UserToken(), ""; got != want {
		t.Fatalf("got %q want %q", got, want)
	}

	tokens.UpdateUserToken("hello")
	if got, want := tokens.UserToken(), "hello"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}

	tokens.UpdateUserToken("world")
	if got, want := tokens.UserToken(), "world"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}

	tokens.UpdateUserToken("")
	if got, want := tokens.UserToken(), ""; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestTokenStore_AgentMasterToken(t *testing.T) {
	t.Parallel()
	tokens := new(TokenStore)

	if got, want := tokens.IsAgentMasterToken(""), false; got != want {
		t.Fatalf("got %v want %v", got, want)
	}
	if got, want := tokens.IsAgentMasterToken("nope"), false; got != want {
		t.Fatalf("got %v want %v", got, want)
	}

	tokens.UpdateAgentMasterToken("master")
	if got, want := tokens.IsAgentMasterToken(""), false; got != want {
		t.Fatalf("got %v want %v", got, want)
	}
	if got, want := tokens.IsAgentMasterToken("nope"), false; got != want {
		t.Fatalf("got %v want %v", got, want)
	}
	if got, want := tokens.IsAgentMasterToken("master"), true; got != want {
		t.Fatalf("got %v want %v", got, want)
	}

	tokens.UpdateAgentMasterToken("another")
	if got, want := tokens.IsAgentMasterToken(""), false; got != want {
		t.Fatalf("got %v want %v", got, want)
	}
	if got, want := tokens.IsAgentMasterToken("nope"), false; got != want {
		t.Fatalf("got %v want %v", got, want)
	}
	if got, want := tokens.IsAgentMasterToken("master"), false; got != want {
		t.Fatalf("got %v want %v", got, want)
	}
	if got, want := tokens.IsAgentMasterToken("another"), true; got != want {
		t.Fatalf("got %v want %v", got, want)
	}
}
