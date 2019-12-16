// +build consul

package consul

import (
	"context"
	"fmt"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
	"testing"
)

func TestConsul_get(t *testing.T) {
	c := &Consul{
		PathPrefix: "skydns",
	}
	c.Client, _ = newConsulClient(defaultAddress, defaulttoken)
	ctx := context.TODO()

	gotR, _, gotMeta, err := c.get(ctx, "skydns/test/skydns_zoneb/dns/apex", false)
	fmt.Println(gotR, gotMeta, err)
	//for _, x := range gotR {
	//	fmt.Println(x.Key, strings.Split(strings.ReplaceAll(string(x.Value), "- ", ""), "\n"))
	//}
	opt := plugin.Options{}
	var w dns.ResponseWriter
	var r *dns.Msg
	state := request.Request{
		Req:  r,
		W:    w,
		Zone: "",
	}

	//zone := plugin.Zones(c.Zones).Matches(state.Name())
	fmt.Println(c.Services(ctx, state, false, opt))
}
