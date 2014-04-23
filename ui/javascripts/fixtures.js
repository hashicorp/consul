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
        "node-10-0-1-102",
        "node-10-0-1-103"
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
        "node-10-0-1-109",
        "node-10-0-1-102",
        "node-10-0-1-103"
      ]
    },
]

fixtures.dcs = ['nyc1', 'sf1', 'sg1']

localStorage.setItem("current_dc", fixtures.dcs[0]);
