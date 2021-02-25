// rootURL in production equals `{{.ContentPath}}` and therefore is replaced
// with the value of -ui-content-path. During development rootURL uses the
// value as set in environment.js

module.exports = ({ appName, environment, rootURL, config }) => `
  <!-- CONSUL_VERSION: ${config.CONSUL_VERSION} -->
  <link rel="icon" type="image/png" href="${rootURL}assets/favicon-32x32.png" sizes="32x32">
  <link rel="icon" type="image/png" href="${rootURL}assets/favicon-16x16.png" sizes="16x16">
  <link integrity="" rel="stylesheet" href="${rootURL}assets/vendor.css">
  <link integrity="" rel="stylesheet" href="${rootURL}assets/${appName}.css">
  ${
    environment === 'development' ? `<link rel="stylesheet" href="${rootURL}assets/debug.css">` : ``
  }
  ${
    environment === 'test' ? `<link rel="stylesheet" href="${rootURL}assets/test-support.css">` : ``
  }
`;
