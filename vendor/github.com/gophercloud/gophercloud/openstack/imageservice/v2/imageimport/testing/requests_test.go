package testing

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/gophercloud/gophercloud/openstack/imageservice/v2/imageimport"
	th "github.com/gophercloud/gophercloud/testhelper"
	fakeclient "github.com/gophercloud/gophercloud/testhelper/client"
)

func TestGet(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()

	th.Mux.HandleFunc("/info/import", func(w http.ResponseWriter, r *http.Request) {
		th.TestMethod(t, r, "GET")
		th.TestHeader(t, r, "X-Auth-Token", fakeclient.TokenID)

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		fmt.Fprintf(w, ImportGetResult)
	})

	validImportMethods := []string{
		string(imageimport.GlanceDirectMethod),
		string(imageimport.WebDownloadMethod),
	}

	s, err := imageimport.Get(fakeclient.ServiceClient()).Extract()
	th.AssertNoErr(t, err)

	th.AssertEquals(t, s.ImportMethods.Description, "Import methods available.")
	th.AssertEquals(t, s.ImportMethods.Type, "array")
	th.AssertDeepEquals(t, s.ImportMethods.Value, validImportMethods)
}
