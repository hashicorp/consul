// rootURL in production equals `{{.ContentPath}}` and therefore is replaced
// with the value of -ui-content-path. During development rootURL uses the
// value as set in environment.js

const read = require('fs').readFileSync;

const hbsRe = /{{(@[a-z]*)}}/g;

const hbs = (path, attrs = {}) =>
  read(`${process.cwd()}/app/components/${path}`)
    .toString()
    .replace('{{yield}}', '')
    .replace(hbsRe, (match, prop) => attrs[prop.substr(1)]);

const BrandLoader = attrs => hbs('brand-loader/index.hbs', attrs);
const Enterprise = attrs => hbs('brand-loader/enterprise.hbs', attrs);

module.exports = ({ appName, environment, rootURL, config }) => `
  <noscript>
      <div style="margin: 0 auto;">
          <h2>JavaScript Required</h2>
          <p>Please enable JavaScript in your web browser to use Consul UI.</p>
      </div>
  </noscript>
${BrandLoader({
  color: '#8E96A3',
  width: config.CONSUL_BINARY_TYPE !== 'oss' && config.CONSUL_BINARY_TYPE !== '' ? `394` : `198`,
  subtitle:
    config.CONSUL_BINARY_TYPE !== 'oss' && config.CONSUL_BINARY_TYPE !== '' ? Enterprise() : ``,
})}
  <script type="application/json" data-consul-ui-config>
${environment === 'production' ? `{{jsonEncode .}}` : JSON.stringify(config.operatorConfig)}
  </script>
  <script type="application/json" data-consul-ui-fs>
  {
    "text-encoding/encoding-indexes.js": "${rootURL}assets/encoding-indexes.js",
    "text-encoding/encoding.js": "${rootURL}assets/encoding.js",
    "css.escape/css.escape.js": "${rootURL}assets/css.escape.js",
    "codemirror/mode/javascript/javascript.js": "${rootURL}assets/codemirror/mode/javascript/javascript.js",
    "codemirror/mode/ruby/ruby.js": "${rootURL}assets/codemirror/mode/ruby/ruby.js",
    "codemirror/mode/yaml/yaml.js": "${rootURL}assets/codemirror/mode/yaml/yaml.js"
  }
  </script>
${
  environment === 'production'
    ? `
{{if .ACLsEnabled}}
  <script data-${appName}-routing src="${rootURL}assets/acls/routes.js"></script>
{{end}}
`
    : `
<script>
  if(document.cookie['CONSUL_ACLS_ENABLED']) {
    const appName = '${appName}';
    const appNameJS = appName.split('-').map((item, i) => i ? \`\${item.substr(0, 1).toUpperCase()}\${item.substr(1)}\` : item).join('');
    const $script = document.createElement('script');
    $script.setAttribute('src', '${rootURL}assets/acls/routes.js');
    $script.dataset[\`\${appNameJS}Routes\`] = null;
    document.body.appendChild($script);
  }
</script>
`
}
  <script src="${rootURL}assets/init.js"></script>
  <script src="${rootURL}assets/vendor.js"></script>
  ${environment === 'test' ? `<script src="${rootURL}assets/test-support.js"></script>` : ``}
  <script src="${rootURL}assets/metrics-providers/consul.js"></script>
  <script src="${rootURL}assets/metrics-providers/prometheus.js"></script>
  ${
    environment === 'production'
      ? `{{ range .ExtraScripts }} <script src="{{.}}"></script> {{ end }}`
      : ``
  }
  <script src="${rootURL}assets/${appName}.js"></script>
  ${environment === 'test' ? `<script src="${rootURL}assets/tests.js"></script>` : ``}
`;
