package command

import (
	"flag"
	"os"
	"testing"
)

const (
	defaultRPC  = "127.0.0.1:8400"
	defaultHTTP = "127.0.0.1:8500"
)

type flagFunc func(f *flag.FlagSet) *string

func getParsedAddr(t *testing.T, addrType, cliVal, envVal string) string {
	var cliFlag, envVar string
	var fn flagFunc
	args := []string{}

	switch addrType {
	case "rpc":
		fn = RPCAddrFlag
		envVar = RPCAddrEnvName
		cliFlag = "-rpc-addr"
	case "http":
		fn = HTTPAddrFlag
		envVar = HTTPAddrEnvName
		cliFlag = "-http-addr"
	default:
		t.Fatalf("unknown address type %s", addrType)
	}

	if cliVal != "" {
		args = append(args, cliFlag+"="+cliVal)
	}

	os.Clearenv()
	if envVal != "" {
		os.Setenv(envVar, envVal)
	}

	cmdFlags := flag.NewFlagSet(addrType, flag.ContinueOnError)
	result := fn(cmdFlags)

	if err := cmdFlags.Parse(args); err != nil {
		t.Fatal("Parse error", err)
	}

	return *result
}

func TestAddrFlag_default(t *testing.T) {
	for a, def := range map[string]string{
		"rpc":  defaultRPC,
		"http": defaultHTTP,
	} {
		res := getParsedAddr(t, a, "", "")

		if res != def {
			t.Fatalf("Expected addr: %s, got: %s", def, res)
		}
	}
}

func TestAddrFlag_onlyEnv(t *testing.T) {
	envAddr := "4.4.4.4:1234"
	for _, a := range []string{"rpc", "http"} {
		res := getParsedAddr(t, a, "", envAddr)

		if res != envAddr {
			t.Fatalf("Expected %s addr: %s, got: %s", a, envAddr, res)
		}
	}
}

func TestAddrFlag_precedence(t *testing.T) {
	cliAddr := "8.8.8.8:8400"
	envAddr := "4.4.4.4:8400"
	for _, a := range []string{"rpc", "http"} {
		res := getParsedAddr(t, a, cliAddr, envAddr)

		if res != cliAddr {
			t.Fatalf("Expected %s addr: %s, got: %s", a, cliAddr, res)
		}
	}
}
