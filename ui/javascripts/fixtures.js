//
// I intentionally am not using ember-data and the fixture
// adapter. I'm not confident the Consul UI API will be compatible
// without a bunch of wrangling, and it's really not enough updating
// of the models to justify the use of such a big component. getJSON
// *should* be enough.
//

window.fixtures = {}

fixtures.services = [
    {
      "Name": "vagrant-cloud-http",
      "Checks": [
        {
          "Name": "serfHealth",
          "Status": "passing"
        },
        {
          "Name": "fooHealth",
          "Status": "critical"
        },
        {
          "Name": "bazHealth",
          "Status": "passing"
        }
      ],
      "Nodes": [
        "node-10-0-1-109",
        "node-10-0-3-84"
      ]
    },
    {
      "Name": "vagrant-share-mux",
      "Checks": [
        {
          "Name": "serfHealth",
          "Status": "passing"
        },
        {
          "Name": "fooHealth",
          "Status": "passing"
        },
        {
          "Name": "bazHealth",
          "Status": "passing"
        }
      ],
      "Nodes": [
        "node-10-0-1-103",
        "node-10-0-1-104"
      ]
    },
]

// This is both of the fixture services full response. We
// would just expect one of these, inside of the top level
// key. We require that key just for the fixture lookup.
fixtures.services_full = {
  "vagrant-cloud-http": [
    // A node
    {
      "ServicePort": 80,
      "ServiceTags": null,
      "ServiceName": "vagrant-cloud-http",
      "ServiceID": "vagrant-cloud-http",
      "Address": "10.0.1.109",
      "Node": "node-10-0-1-109",
      "Checks": [
        {
          "ServiceName": "",
          "ServiceID": "",
          "Notes": "",
          "Status": "critical",
          "Name": "Serf Health Status",
          "CheckID": "serfHealth",
          "Node": "node-10-0-3-83"
        }
      ]
    },
    // A node
    {
      "ServicePort": 80,
      "ServiceTags": null,
      "ServiceName": "vagrant-cloud-http",
      "ServiceID": "vagrant-cloud-http",
      "Address": "10.0.3.83",
      "Node": "node-10-0-3-84",
      "Checks": [
        {
          "ServiceName": "",
          "ServiceID": "",
          "Notes": "",
          "Status": "passing",
          "Name": "Serf Health Status",
          "CheckID": "serfHealth",
          "Node": "node-10-0-3-84"
        }
      ]
    }
  ],
  "vagrant-share-mux": [
    // A node
    {
      "ServicePort": 80,
      "ServiceTags": null,
      "ServiceName": "vagrant-share-mux",
      "ServiceID": "vagrant-share-mux",
      "Address": "10.0.1.104",
      "Node": "node-10-0-1-104",
      "Checks": [
        {
          "ServiceName": "vagrant-share-mux",
          "ServiceID": "vagrant-share-mux",
          "Notes": "",
          "Output": "200 ok",
          "Status": "passing",
          "Name": "Foo Heathly",
          "CheckID": "fooHealth",
          "Node": "node-10-0-1-104"
        }
      ]
    },
    // A node
    {
      "ServicePort": 80,
      "ServiceTags": null,
      "ServiceName": "vagrant-share-mux",
      "ServiceID": "vagrant-share-mux",
      "Address": "10.0.1.103",
      "Node": "node-10-0-1-103",
      "Checks": [
        {
          "ServiceName": "",
          "ServiceID": "",
          "Notes": "",
          "Output": "foobar baz",
          "Status": "passing",
          "Name": "Baz Status",
          "CheckID": "bazHealth",
          "Node": "node-10-0-1-103"
        },
        {
          "ServiceName": "",
          "ServiceID": "",
          "Notes": "",
          "Output": "foobar baz",
          "Status": "passing",
          "Name": "Serf Health Status",
          "CheckID": "serfHealth",
          "Node": "node-10-0-1-103"
        }
      ]
    }
  ]
}

fixtures.dcs = ['nyc1', 'sf1', 'sg1']

localStorage.setItem("current_dc", fixtures.dcs[0]);
