/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

(services => services({
  "component:consul/partition/selector": {
    "class": "consul-ui/components/consul/partition/selector"
  }
}))(
  (json, data = (typeof document !== 'undefined' ? document.currentScript.dataset : module.exports)) => {
    data[`services`] = JSON.stringify(json);
  }
);
