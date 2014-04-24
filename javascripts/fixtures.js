//
// I intentionally am not using ember-data and the fixture
// adapter. I'm not confident the Consul UI API will be compatible
// without a bunch of wrangling, and it's really not enough updating
// of the models to justify the use of such a big component. getJSON
// *should* be enough.
//

window.fixtures = {}

//
// The array route, i.e /ui/services, should return _all_ services
// in the DC
//
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

//
// This one is slightly more complicated to allow more UI interaction.
// It represents the route /ui/services/<service> BUT it's what is
// BELOW the top-level key.
//
// So, what is actually returned should be similar to the /catalog/service/<service>
// endpoint.
fixtures.services_full = {
  "vagrant-cloud-http":
  // This array is what is actually expected from the API.
  [
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


fixtures.nodes = [
    {
      "Address": "10.0.1.109",
      "Name": "node-10-0-1-109",
      "Services": [
          "vagrant-cloud-worker",
          "vagrant-cloud-http"
      ],
      "Checks": [
        {
          "Name": "serfHealth",
          "status": "passing"
        },
        {
          "Name": "fooHealth",
          "status": "critical"
        },
        {
          "Name": "bazHealth",
          "status": "passing"
        }
      ]
    },
    {
      "Address": "10.0.1.102",
      "Name": "node-10-0-1-102",
      "Services": [
          "vagrant-cloud-worker",
          "vagrant-cloud-http"
      ],
      "Checks": [
        {
          "Name": "serfHealth",
          "status": "passing"
        },
        {
          "Name": "fooHealth",
          "status": "critical"
        },
        {
          "Name": "bazHealth",
          "status": "passing"
        }
     ],

    },
    {
      "Address": "10.0.1.103",
      "Name": "node-10-0-1-103",
      "Services": [
          "vagrant-cloud-worker",
          "vagrant-cloud-http"
      ],
      "Checks": [
        {
          "Name": "serfHealth",
          "status": "passing"
        },
        {
          "Name": "fooHealth",
          "status": "critical"
        },
        {
          "Name": "bazHealth",
          "status": "passing"
        }
      ]
    }
]

fixtures.nodes_full = {
  "node-10-0-1-109": {
    "Services": [
      {
        "Port": 0,
        "Tags": null,
        "Service": "vagrant-cloud-http",
        "ID": "vagrant-cloud-http"
      },
      {
        "Port": 80,
        "Tags": null,
        "Service": "vagrant-cloud-http",
        "ID": "vagrant-cloud-http"
      }
    ],
    "Node": {
      "Address": "10.0.1.109",
      "Node": "node-10-0-1-109"
    },
    "Checks": [
      {
        "ServiceName": "",
        "ServiceID": "",
        "Notes": "Checks the status of the serf agent",
        "Status": "passing",
        "Name": "Serf Health Status",
        "CheckID": "serfHealth",
        "Node": "node-10-0-1-109"
      }
    ]
  },
  "node-10-0-1-102": {
    "Services": [
      {
        "Port": 0,
        "Tags": null,
        "Service": "vagrant-cloud-http",
        "ID": "vagrant-cloud-http"
      },
      {
        "Port": 80,
        "Tags": null,
        "Service": "vagrant-cloud-http",
        "ID": "vagrant-cloud-http"
      }
    ],
    "Node": {
      "Address": "10.0.1.102",
      "Node": "node-10-0-1-102"
    },
    "Checks": [
        {
          "ServiceName": "",
          "ServiceID": "",
          "Notes": "",
          "Output": "foobar baz",
          "Status": "passing",
          "Name": "Baz Status",
          "CheckID": "bazHealth",
          "Node": "node-10-0-1-102"
        },
        {
          "ServiceName": "",
          "ServiceID": "",
          "Notes": "",
          "Output": "foobar baz",
          "Status": "passing",
          "Name": "Serf Health Status",
          "CheckID": "serfHealth",
          "Node": "node-10-0-1-102"
        }
    ]
  }
}

fixtures.dcs = ['nyc1', 'sf1', 'sg1']

localStorage.setItem("current_dc", fixtures.dcs[0]);
