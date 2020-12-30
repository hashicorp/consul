module.exports = ({ appName, environment, rootURL, config }) => `
  <!-- CONSUL_VERSION: ${config.CONSUL_VERSION} -->
  <meta name="consul-ui/ui_config" content="${
    environment === 'production'
      ? `{{ jsonEncodeAndEscape .UIConfig }}`
      : escape(JSON.stringify({}))
  }" />

  <link rel="icon" type="image/png" href="${rootURL}assets/favicon-32x32.png" sizes="32x32">
  <link rel="icon" type="image/png" href="${rootURL}assets/favicon-16x16.png" sizes="16x16">
  <link integrity="" rel="stylesheet" href="${rootURL}assets/vendor.css">
  <link integrity="" rel="stylesheet" href="${rootURL}assets/${appName}.css">
  ${
    environment === 'test' ? `<link rel="stylesheet" href="${rootURL}assets/test-support.css">` : ``
  }
`;
