/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

(routes =>
  routes({
    ['oauth-provider-debug']: {
      _options: {
        path: '/oauth-provider-debug',
        queryParams: {
          redirect_uri: 'redirect_uri',
          response_type: 'response_type',
          scope: 'scope',
        },
      },
    },
  }))(
  (
    json,
    data = typeof document !== 'undefined' ? document.currentScript.dataset : module.exports
  ) => {
    data[`routes`] = JSON.stringify(json);
  }
);
