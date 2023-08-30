/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

((routes) =>
  routes({
    dc: {
      peers: {
        _options: {
          path: "/peers",
        },
        index: {
          _options: {
            path: "/",
            queryParams: {
              sortBy: "sort",
              state: "state",
              searchproperty: {
                as: "searchproperty",
                empty: [["Name", "ID"]],
              },
              search: {
                as: "filter",
                replace: true,
              },
            },
          },
        },
        show: {
          _options: {
            path: "/:name",
          },
          imported: {
            _options: {
              path: "/imported-services",
              queryParams: {
                sortBy: "sort",
                status: "status",
                source: "source",
                kind: "kind",
                searchproperty: {
                  as: "searchproperty",
                  empty: [["Name", "Tags"]],
                },
                search: {
                  as: "filter",
                  replace: true,
                },
              },
            },
          },
          exported: {
            _options: {
              path: "/exported-services",
              queryParams: {
                search: {
                  as: "filter",
                  replace: true,
                },
              },
            },
          },
          addresses: {
            _options: {
              path: "/addresses",
            },
          },
        },
      },
    },
  }))(
  (
    json,
    data = typeof document !== "undefined"
      ? document.currentScript.dataset
      : module.exports
  ) => {
    data[`routes`] = JSON.stringify(json);
  }
);
