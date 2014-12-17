package command

import (
	"flag"
	"os"
	"testing"
)

const defaultRPC = "127.0.0.1:8400"

func getParsedRPC(t *testing.T, cliRPC, envRPC string) string {
	args := []string{}

	if cliRPC != "" {
		args = append(args, "-rpc-addr="+cliRPC)
	}

	os.Clearenv()
	if envRPC != "" {
		os.Setenv(RPCAddrEnvName, envRPC)
	}

	cmdFlags := flag.NewFlagSet("rpc", flag.ContinueOnError)
	rpc := RPCAddrFlag(cmdFlags)

	if err := cmdFlags.Parse(args); err != nil {
		t.Fatal("Parse error", err)
	}

	return *rpc
}

func TestRPCAddrFlag_default(t *testing.T) {
	rpc := getParsedRPC(t, "", "")

	if rpc != defaultRPC {
		t.Fatalf("Expected rpc addr: %s, got: %s", defaultRPC, rpc)
	}
}

func TestRPCAddrFlag_onlyEnv(t *testing.T) {
	envRPC := "4.4.4.4:8400"
	rpc := getParsedRPC(t, "", envRPC)

	if rpc != envRPC {
		t.Fatalf("Expected rpc addr: %s, got: %s", envRPC, rpc)
	}
}

func TestRPCAddrFlag_precedence(t *testing.T) {
	cliRPC := "8.8.8.8:8400"
	envRPC := "4.4.4.4:8400"

	rpc := getParsedRPC(t, cliRPC, envRPC)

	if rpc != cliRPC {
		t.Fatalf("Expected rpc addr: %s, got: %s", cliRPC, rpc)
	}
}
