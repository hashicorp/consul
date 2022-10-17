package assert

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/consul/sdk/testutil/retry"
)

func HTTPResponseContains(t *testing.T, ip string, port int, phrase string) {
	failer := func() *retry.Timer {
		return &retry.Timer{Timeout: defaultTimeout, Wait: defaultWait}
	}

	client := http.DefaultClient
	url := fmt.Sprintf("http://%s:%d", ip, port)

	retry.RunWith(failer(), t, func(r *retry.R) {

		reader := strings.NewReader(phrase)
		res, err := client.Post(url, "text/plain", reader)
		if err != nil {
			r.Fatal("could not make call to service ", url)
		}

		body, err := io.ReadAll(res.Body)
		if err != nil {
			r.Fatal("could not read response body ", url)
		}

		if !strings.Contains(string(body), phrase) {
			r.Fatal("received an incorrect response ", body)
		}
	})

}
