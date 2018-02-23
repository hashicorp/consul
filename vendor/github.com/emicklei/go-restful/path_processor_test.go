package restful

import "testing"

func TestMatchesPath_OneParam(t *testing.T) {
	params := doExtractParams("/from/{source}", 2, "/from/here", t)
	if params["source"] != "here" {
		t.Errorf("parameter mismatch here")
	}
}

func TestMatchesPath_Slash(t *testing.T) {
	params := doExtractParams("/", 0, "/", t)
	if len(params) != 0 {
		t.Errorf("expected empty parameters")
	}
}

func TestMatchesPath_SlashNonVar(t *testing.T) {
	params := doExtractParams("/any", 1, "/any", t)
	if len(params) != 0 {
		t.Errorf("expected empty parameters")
	}
}

func TestMatchesPath_TwoVars(t *testing.T) {
	params := doExtractParams("/from/{source}/to/{destination}", 4, "/from/AMS/to/NY", t)
	if params["source"] != "AMS" {
		t.Errorf("parameter mismatch AMS")
	}
}

func TestMatchesPath_VarOnFront(t *testing.T) {
	params := doExtractParams("{what}/from/{source}/", 3, "who/from/SOS/", t)
	if params["source"] != "SOS" {
		t.Errorf("parameter mismatch SOS")
	}
}

func TestExtractParameters_EmptyValue(t *testing.T) {
	params := doExtractParams("/fixed/{var}", 2, "/fixed/", t)
	if params["var"] != "" {
		t.Errorf("parameter mismatch var")
	}
}

func doExtractParams(routePath string, size int, urlPath string, t *testing.T) map[string]string {
	r := Route{Path: routePath}
	r.postBuild()
	if len(r.pathParts) != size {
		t.Fatalf("len not %v %v, but %v", size, r.pathParts, len(r.pathParts))
	}
	pathProcessor := defaultPathProcessor{}
	return pathProcessor.ExtractParameters(&r, nil, urlPath)
}
