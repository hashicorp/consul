module.exports = ({ appName, environment, rootURL, config }) => `
  <!-- CONSUL_VERSION: ${config.CONSUL_VERSION} -->
  <script>
    var setConfig = function(appName, config) {
      var $meta = document.querySelector('meta[name="' + appName + '/config/environment"]');
      var defaultConfig = JSON.parse(decodeURIComponent($meta.getAttribute('content')));
      (
        function set(blob, config) {
          Object.keys(config).forEach(
            function(key) {
              var value = config[key];
              if(Object.prototype.toString.call(value) === '[object Object]') {
                set(blob[key], config[key]);
              } else {
                blob[key] = config[key];
              }
            }
          );
        }
      )(defaultConfig, config);
      $meta.setAttribute('content', encodeURIComponent(JSON.stringify(defaultConfig)));
    }
    setConfig(
      '${appName}',
      {
        rootURL: '${rootURL}',
        CONSUL_ACLS_ENABLED: ${config.CONSUL_ACLS_ENABLED},
        CONSUL_NSPACES_ENABLED: ${config.CONSUL_NSPACES_ENABLED},
        CONSUL_SSO_ENABLED: ${config.CONSUL_SSO_ENABLED}
      }
    );
  </script>
  <link rel="icon" type="image/png" href="${rootURL}assets/favicon-32x32.png" sizes="32x32">
  <link rel="icon" type="image/png" href="${rootURL}assets/favicon-16x16.png" sizes="16x16">
  <link integrity="" rel="stylesheet" href="${rootURL}assets/vendor.css">
  <link integrity="" rel="stylesheet" href="${rootURL}assets/${appName}.css">
  ${
    environment === 'test' ? `<link rel="stylesheet" href="${rootURL}assets/test-support.css">` : ``
  }
`;
