/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

(routes => routes({
  dc: {
    nodes: {
      show: {
        sessions: {
          _options: { path: '/lock-sessions' },
        },
      },
    },
  },
}))(
  (json, data = (typeof document !== 'undefined' ? document.currentScript.dataset : module.exports)) => {
    data[`routes`] = JSON.stringify(json);
  }
);
