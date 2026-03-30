/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

(services =>
  services({
    'route:basic': {
      class: 'consul-ui/routing/route',
    },
    'service:intl': {
      class: 'consul-ui/services/i18n',
    },
    'service:state': {
      class: 'consul-ui/services/state-with-charts',
    },
    'auth-provider:oidc-with-url': {
      class: 'consul-ui/services/auth-providers/oauth2-code-with-url-provider',
    },
    'component:consul/partition/selector': {
      class: 'consul-ui/components/consul/partition/selector',
    },
    'component:consul/peer/selector': {
      class: 'consul-ui/components/consul/peer/selector',
    },
    'component:consul/hcp/home': {
      class: '@glimmer/component',
    },
  }))(
  (
    json,
    data = typeof document !== 'undefined' ? document.currentScript.dataset : module.exports
  ) => {
    data[`services`] = JSON.stringify(json);
  }
);
