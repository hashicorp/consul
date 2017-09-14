package msg

import "testing"

func TestPath(t *testing.T) {
	for _, path := range []string{"mydns", "skydns"} {
		result := Path("service.staging.skydns.local.", path)
		if result != "/"+path+"/local/skydns/staging/service" {
			t.Errorf("Failure to get domain's path with prefix: %s", result)
		}
	}
}
