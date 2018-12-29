package testing

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/services"
	th "github.com/gophercloud/gophercloud/testhelper"
	"github.com/gophercloud/gophercloud/testhelper/client"
)

// ServiceListBody is sample response to the List call
const ServiceListBody = `
{
    "services": [
        {
            "id": 1,
            "binary": "nova-scheduler",
            "disabled_reason": "test1",
            "host": "host1",
            "state": "up",
            "status": "disabled",
            "updated_at": "2012-10-29T13:42:02.000000",
            "forced_down": false,
            "zone": "internal"
        },
        {
            "id": 2,
            "binary": "nova-compute",
            "disabled_reason": "test2",
            "host": "host1",
            "state": "up",
            "status": "disabled",
            "updated_at": "2012-10-29T13:42:05.000000",
            "forced_down": false,
            "zone": "nova"
        },
        {
            "id": 3,
            "binary": "nova-scheduler",
            "disabled_reason": null,
            "host": "host2",
            "state": "down",
            "status": "enabled",
            "updated_at": "2012-09-19T06:55:34.000000",
            "forced_down": false,
            "zone": "internal"
        },
        {
            "id": 4,
            "binary": "nova-compute",
            "disabled_reason": "test4",
            "host": "host2",
            "state": "down",
            "status": "disabled",
            "updated_at": "2012-09-18T08:03:38.000000",
            "forced_down": false,
            "zone": "nova"
        }
    ]
}
`

// First service from the ServiceListBody
var FirstFakeService = services.Service{
	Binary:         "nova-scheduler",
	DisabledReason: "test1",
	Host:           "host1",
	ID:             1,
	State:          "up",
	Status:         "disabled",
	UpdatedAt:      time.Date(2012, 10, 29, 13, 42, 2, 0, time.UTC),
	Zone:           "internal",
}

// Second service from the ServiceListBody
var SecondFakeService = services.Service{
	Binary:         "nova-compute",
	DisabledReason: "test2",
	Host:           "host1",
	ID:             2,
	State:          "up",
	Status:         "disabled",
	UpdatedAt:      time.Date(2012, 10, 29, 13, 42, 5, 0, time.UTC),
	Zone:           "nova",
}

// Third service from the ServiceListBody
var ThirdFakeService = services.Service{
	Binary:         "nova-scheduler",
	DisabledReason: "",
	Host:           "host2",
	ID:             3,
	State:          "down",
	Status:         "enabled",
	UpdatedAt:      time.Date(2012, 9, 19, 6, 55, 34, 0, time.UTC),
	Zone:           "internal",
}

// Fourth service from the ServiceListBody
var FourthFakeService = services.Service{
	Binary:         "nova-compute",
	DisabledReason: "test4",
	Host:           "host2",
	ID:             4,
	State:          "down",
	Status:         "disabled",
	UpdatedAt:      time.Date(2012, 9, 18, 8, 3, 38, 0, time.UTC),
	Zone:           "nova",
}

// HandleListSuccessfully configures the test server to respond to a List request.
func HandleListSuccessfully(t *testing.T) {
	th.Mux.HandleFunc("/os-services", func(w http.ResponseWriter, r *http.Request) {
		th.TestMethod(t, r, "GET")
		th.TestHeader(t, r, "X-Auth-Token", client.TokenID)

		w.Header().Add("Content-Type", "application/json")
		fmt.Fprintf(w, ServiceListBody)
	})
}
