#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


set -euo pipefail

function docker_consul {
  local DC=$1
  shift 1
  docker_exec envoy_consul-${DC}_1 "$@"
}

function upsert_config_entry {
  local DC="$1"
  local BODY="$2"

  echo "$BODY" | docker_consul "$DC" config write -
}

function docker_exec {
  if ! docker.exe exec -i "$@"; then
    echo "Failed to execute: docker exec -i $@" 1>&2
    return 1
  fi
}

upsert_config_entry primary '
kind = "proxy-defaults"
name = "global"
config {
  protocol = "http"
}
'

upsert_config_entry primary '
kind           = "service-resolver"
name           = "s2"
default_subset = "v1"
subsets = {
  "v1" = {
    filter = "Service.Meta.version == v1"
  }
  "v2" = {
    filter = "Service.Meta.version == v2"
  }
}
'

upsert_config_entry primary '
kind = "service-router"
name = "s2"
routes = [
  {
    match { http { path_exact = "/exact/debug" } }
    destination {
      service_subset = "v2"
      prefix_rewrite = "/debug"
    }
  },
  {
    match { http { path_exact = "/exact-alt/debug" } }
    destination {
      service_subset = "v1"
      prefix_rewrite = "/debug"
    }
  },
  {
    match { http { path_prefix = "/prefix/" } }
    destination {
      service_subset = "v2"
      prefix_rewrite = "/"
    }
  },
  {
    match { http { path_prefix = "/prefix-alt/" } }
    destination {
      service_subset = "v1"
      prefix_rewrite = "/"
    }
  },
  {
    match { http {
      path_regex = "/deb[ug]{2}"
      header = [{
        name  = "x-test-debug"
        exact = "regex-path"
      }]
    } }
    destination {
      service_subset           = "v2"
      retry_on_connect_failure = true       # TODO: test
      retry_on                 = ["reset"]  # TODO: test
      retry_on_status_codes    = [500, 512] # TODO: test
    }
  },
  {
    match { http {
      path_exact = "/hdr-present/debug"
      header = [
        {
          name    = "x-test-debug"
          present = true
        },
      ]
    } },
    destination {
      service_subset = "v2"
      prefix_rewrite = "/debug"
    }
  },
  {
    match { http {
      path_exact = "/hdr-present/debug"
      header = [
        {
          name    = "x-test-debug"
          present = true
          invert  = true
        },
      ]
    } },
    destination {
      service_subset = "v1"
      prefix_rewrite = "/debug"
    }
  },
  {
    match { http {
      path_exact = "/hdr-exact/debug"
      header = [
        {
          name  = "x-test-debug"
          exact = "exact"
        },
      ]
    } },
    destination {
      service_subset = "v2"
      prefix_rewrite = "/debug"
    }
  },
  {
    match { http {
      path_exact = "/hdr-exact/debug"
      header = [
        {
          name  = "x-test-debug"
          exact = "exact-alt"
        },
      ]
    } },
    destination {
      service_subset = "v1"
      prefix_rewrite = "/debug"
    }
  },
  {
    match { http {
      path_exact = "/hdr-prefix/debug"
      header = [
        {
          name   = "x-test-debug"
          prefix = "prefi"
        },
    ] } },
    destination {
      service_subset = "v2"
      prefix_rewrite = "/debug"
    }
  },
  {
    match { http {
      path_exact = "/hdr-prefix/debug"
      header = [
        {
          name   = "x-test-debug"
          prefix = "alt-prefi"
        },
    ] } },
    destination {
      service_subset = "v1"
      prefix_rewrite = "/debug"
    }
  },
  {
    match { http {
      path_exact = "/hdr-suffix/debug"
      header = [
        {
          name   = "x-test-debug"
          suffix = "uffix"
        },
      ]
    } },
    destination {
      service_subset = "v2"
      prefix_rewrite = "/debug"
    }
  },
  {
    match { http {
      path_exact = "/hdr-suffix/debug"
      header = [
        {
          name   = "x-test-debug"
          suffix = "uffix-alt"
        },
      ]
    } },
    destination {
      service_subset = "v1"
      prefix_rewrite = "/debug"
    }
  },
  {
    match { http {
      path_exact = "/hdr-regex/debug"
      header = [
        {
          name  = "x-test-debug"
          regex = "reg[ex]{2}"
        },
      ]
    } },
    destination {
      service_subset = "v2"
      prefix_rewrite = "/debug"
    }
  },
  {
    match { http {
      path_exact = "/hdr-regex/debug"
      header = [
        {
          name  = "x-test-debug"
          regex = "reg[ex]{3}"
        },
      ]
    } },
    destination {
      service_subset = "v1"
      prefix_rewrite = "/debug"
    }
  },
  {
    match { http {
      path_exact = "/hdr-invert/debug"
      header = [
        {
          name   = "x-test-debug"
          exact  = "not-this"
          invert = true
        },
      ],
    } },
    destination {
      service_subset = "v2"
      prefix_rewrite = "/debug"
    }
  },
  {
    match { http {
      path_exact = "/qp-present/debug"
      query_param = [
        {
          name    = "env"
          present = true
        },
      ],
    } },
    destination {
      service_subset = "v2"
      prefix_rewrite = "/debug"
    }
  },
  {
    match { http {
      path_exact = "/qp-exact/debug"
      query_param = [
        {
          name  = "env"
          exact = "dump"
        },
      ],
    } },
    destination {
      service_subset = "v2"
      prefix_rewrite = "/debug"
    }
  },
  {
    match { http {
      path_exact = "/qp-regex/debug"
      query_param = [
        {
          name  = "env"
          regex = "du[mp]{2}"
        },
      ],
    } },
    destination {
      service_subset = "v2"
      prefix_rewrite = "/debug"
    }
  },
  {
    match { http {
      path_exact = "/method-match/debug"
      methods    = ["GET", "PUT"]
    } },
    destination {
      service_subset = "v2"
      prefix_rewrite = "/debug"
    }
  },
  {
    match { http {
      path_exact = "/header-manip/debug"
    } },
    destination {
      service_subset = "v2"
      prefix_rewrite = "/debug"
      request_headers {
        set {
          x-foo = "request-bar"
        }
        remove = ["x-bad-req"]
      }
    }
  },
  {
    match { http {
      path_exact = "/header-manip/echo"
    } },
    destination {
      service_subset = "v2"
      prefix_rewrite = "/"
      response_headers {
        add {
          x-foo = "response-bar"
        }
        remove = ["x-bad-resp"]
      }
    }
  },
]
'

register_services primary

# s2 is retained just to have a honeypot for bad envoy configs to route into
gen_envoy_bootstrap s1 19000
gen_envoy_bootstrap s2-v1 19001
gen_envoy_bootstrap s2-v2 19002
gen_envoy_bootstrap s2 19003
