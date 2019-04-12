package structs

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"testing"

	proto "github.com/gogo/protobuf/proto"
	msgpack "github.com/hashicorp/go-msgpack/codec"
	"github.com/stretchr/testify/require"
)

const dataHealthEntry = `
{
	"Node": {
	  "ID": "9c74bfd0-ef59-21e1-26d1-298a43f8e032",
	  "Node": "some-host.dc.org",
	  "Address": "10.1.2.3",
	  "Datacenter": "par",
	  "TaggedAddresses": {
		"lan": "10.1.2.3",
		"wan": "10.1.2.3"
	  },
	  "Meta": {
		"meta-aaaaaaa-1": "value-aaaaaa",
		"meta-aaaaaaa-2": "value-aaaaaa",
		"meta-aaaaaaa-3": "value-aaaaaa",
		"meta-aaaaaaa-4": "value-aaaaaa",
		"meta-aaaaaaa-5": "value-aaaaaa",
		"meta-aaaaaaa-6": "value-aaaaaa"
	  },
	  "CreateIndex": 2371038486,
	  "ModifyIndex": 2371046408
	},
	"Service": {
	  "ID": "service-id",
	  "Service": "service-name",
	  "Tags": [
		"tag-aaaaaaaaa-1",
		"tag-aaaaaaaaa-2",
		"tag-aaaaaaaaa-3",
		"tag-aaaaaaaaa-4",
		"tag-aaaaaaaaa-5",
		"tag-aaaaaaaaa-6",
		"tag-aaaaaaaaa-7"
	  ],
	  "Address": "",
	  "Meta": {
		"meta-aaaaaaa-1": "value-aaaaaa",
		"meta-aaaaaaa-2": "value-aaaaaa",
		"meta-aaaaaaa-3": "value-aaaaaa",
		"meta-aaaaaaa-4": "value-aaaaaa",
		"meta-aaaaaaa-5": "value-aaaaaa",
		"meta-aaaaaaa-6": "value-aaaaaa"
	  },
	  "Port": 88888,
	  "Weights": {
		"Passing": 10,
		"Warning": 2
	  },
	  "EnableTagOverride": false,
	  "ProxyDestination": "",
	  "CreateIndex": 2371038486,
	  "ModifyIndex": 2371038486
	},
	"Checks": [
	  {
		"Node": "some-host.dc.org",
		"CheckID": "serfHealth",
		"Name": "Serf Health Status",
		"Status": "passing",
		"Notes": "",
		"Output": "Agent alive and reachable",
		"ServiceID": "",
		"ServiceName": "",
		"ServiceTags": [],
		"Definition": {},
		"CreateIndex": 2371038486,
		"ModifyIndex": 2371038486
	  },
	  {
		"Node": "some-host.dc.org",
		"CheckID": "service:service-id:1",
		"Name": "service:service-id:1",
		"Status": "passing",
		"Notes": "OK",
		"Output": "OK",
		"ServiceID": "service-id",
		"ServiceName": "service-name",
		"Tags": [
			"tag-aaaaaaaaa-1",
			"tag-aaaaaaaaa-2",
			"tag-aaaaaaaaa-3",
			"tag-aaaaaaaaa-4",
			"tag-aaaaaaaaa-5",
			"tag-aaaaaaaaa-6",
			"tag-aaaaaaaaa-7"
		],
		"Definition": {},
		"CreateIndex": 2371038486,
		"ModifyIndex": 2371038486
	  },
	  {
		"Node": "some-host.dc.org",
		"CheckID": "service:service-id:2",
		"Name": "service:service-id:2",
		"Status": "passing",
		"Notes": "OK",
		"Output": "OK",
		"ServiceID": "service-id",
		"ServiceName": "service-name",
		"Tags": [
			"tag-aaaaaaaaa-1",
			"tag-aaaaaaaaa-2",
			"tag-aaaaaaaaa-3",
			"tag-aaaaaaaaa-4",
			"tag-aaaaaaaaa-5",
			"tag-aaaaaaaaa-6",
			"tag-aaaaaaaaa-7"
		],
		"Definition": {},
		"CreateIndex": 2371038486,
		"ModifyIndex": 2371038486
	  },
	  {
		"Node": "some-host.dc.org",
		"CheckID": "service:service-id:3",
		"Name": "service:service-id:3",
		"Status": "passing",
		"Notes": "OK",
		"Output": "OK",
		"ServiceID": "service-id",
		"ServiceName": "service-name",
		"Tags": [
			"tag-aaaaaaaaa-1",
			"tag-aaaaaaaaa-2",
			"tag-aaaaaaaaa-3",
			"tag-aaaaaaaaa-4",
			"tag-aaaaaaaaa-5",
			"tag-aaaaaaaaa-6",
			"tag-aaaaaaaaa-7"
		],
		"Definition": {},
		"CreateIndex": 2371038486,
		"ModifyIndex": 2371038486
	  }
	]
  }
`

func BenchmarkIndexedCheckserviceNodesEncoding_Proto(b *testing.B) {
	entry := CheckServiceNode{}
	err := json.Unmarshal([]byte(dataHealthEntry), &entry)
	require.Nil(b, err)

	for _, size := range []int{10, 100, 500, 1000} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			data := &IndexedCheckServiceNodes{
				Nodes: make([]CheckServiceNode, size),
			}
			for i := 0; i < size; i++ {
				data.Nodes[i] = entry
			}

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				res, err := proto.Marshal(data)
				require.Nil(b, err)
				require.True(b, len(res) > 0)
			}
		})
	}
}

func BenchmarkIndexedCheckserviceNodesEncoding_MsgPack(b *testing.B) {
	entry := CheckServiceNode{}
	err := json.Unmarshal([]byte(dataHealthEntry), &entry)
	require.Nil(b, err)

	for _, size := range []int{10, 100, 500, 1000} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			data := &IndexedCheckServiceNodes{
				Nodes: make([]CheckServiceNode, size),
			}
			for i := 0; i < size; i++ {
				data.Nodes[i] = entry
			}

			codec := msgpack.NewEncoder(ioutil.Discard, &msgpack.MsgpackHandle{})

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				err := codec.Encode(data)
				require.Nil(b, err)
			}
		})
	}
}
