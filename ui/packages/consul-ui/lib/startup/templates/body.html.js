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

module.exports = ({ appName, environment, rootURL, config, env }) => `
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
    "codemirror/mode/yaml/yaml.js": "${rootURL}assets/codemirror/mode/yaml/yaml.js",
    "codemirror/mode/xml/xml.js": "${rootURL}assets/codemirror/mode/xml/xml.js"
  }
  </script>
  <script src="${rootURL}assets/consul-ui/services.js"></script>
  <script src="${rootURL}assets/consul-ui/routes.js"></script>
  <script src="${rootURL}assets/consul-lock-sessions/routes.js"></script>
${
  environment === 'development' || environment === 'staging'
    ? `
  <script src="${rootURL}assets/consul-ui/services-debug.js"></script>
  <script src="${rootURL}assets/consul-ui/routes-debug.js"></script>
`
    : ``
}
${
  environment === 'production'
    ? `
{{if .ACLsEnabled}}
  <script src="${rootURL}assets/consul-acls/routes.js"></script>
{{end}}
{{if .PeeringEnabled}}
  <script src="${rootURL}assets/consul-peerings/services.js"></script>
  <script src="${rootURL}assets/consul-peerings/routes.js"></script>
{{end}}
{{if .PartitionsEnabled}}
  <script src="${rootURL}assets/consul-partitions/services.js"></script>
  <script src="${rootURL}assets/consul-partitions/routes.js"></script>
{{end}}
{{if .NamespacesEnabled}}
  <script src="${rootURL}assets/consul-nspaces/routes.js"></script>
{{end}}
{{if .HCPEnabled}}
  <script src="${rootURL}assets/consul-hcp/routes.js"></script>
{{end}}
`
    : `
<script>
(
  function(get, obj) {
    Object.entries(obj).forEach(([key, value]) => {
      if(get(key) || (key === 'CONSUL_NSPACES_ENABLE' && ${
        env('CONSUL_NSPACES_ENABLED') === '1' ? `true` : `false`
      })) {
        document.write(\`\\x3Cscript src="${rootURL}assets/\${value}/services.js">\\x3C/script>\`);
        document.write(\`\\x3Cscript src="${rootURL}assets/\${value}/routes.js">\\x3C/script>\`);
      }
    });
  }
)(
  key => document.cookie.split('; ').find(item => item.startsWith(\`\${key}=\`)),
  {
    'CONSUL_ACLS_ENABLE': 'consul-acls',
    'CONSUL_PEERINGS_ENABLE': 'consul-peerings',
    'CONSUL_PARTITIONS_ENABLE': 'consul-partitions',
    'CONSUL_NSPACES_ENABLE': 'consul-nspaces',
    'CONSUL_HCP_ENABLE': 'consul-hcp'
  }
);
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
