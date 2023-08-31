/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

// rootURL in production equals `{{.ContentPath}}` and therefore is replaced
// with the value of -ui-content-path. During development rootURL uses the
// value as set in environment.js

module.exports = ({ appName, environment, rootURL, config }) => `
  <!-- CONSUL_VERSION: ${config.CONSUL_VERSION} -->
  <link rel="icon" href="${rootURL}assets/favicon.ico">
  <link rel="icon" href="${rootURL}assets/favicon.svg" type="image/svg+xml">
  <link rel="apple-touch-icon" href="${rootURL}assets/apple-touch-icon.png">
  <link integrity="" rel="stylesheet" href="${rootURL}assets/vendor.css">
  <link integrity="" rel="stylesheet" href="${rootURL}assets/${appName}.css">
  ${
    environment === 'test' ? `<link rel="stylesheet" href="${rootURL}assets/test-support.css">` : ``
  }
`;
