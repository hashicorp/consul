import { runInDebug } from '@ember/debug';
// 'environment' getter
// there are currently 3 levels of environment variables:
// 1. Those that can be set by the user by setting localStorage values
// 2. Those that can be set by the operator either via ui_config, or inferring
// from other server type properties (protocol)
// 3. Those that can be set only during development by adding cookie values
// via the browsers Web Inspector, or via the browsers hash (#COOKIE_NAME=1),
// which is useful for showing the UI with various settings enabled/disabled
export default function (config = {}, win = window, doc = document) {
  // look at the hash in the URL and transfer anything after the hash into
  // cookies to enable linking of the UI with various settings enabled
  runInDebug(() => {
    const cookies = function (str) {
      return str
        .split(';')
        .map((item) => item.trim())
        .filter((item) => item !== '')
        .filter((item) => item.split('=').shift().startsWith('CONSUL_'));
    };

    // Define the function that reads in "Scenarios", parse and set cookies and set theme if specified.
    // See https://github.com/hashicorp/consul/blob/main/ui/packages/consul-ui/docs/bookmarklets.mdx
    win['Scenario'] = function (str = '') {
      if (str.length > 0) {
        cookies(str).forEach((item) => {
          // this current outlier is the only one that
          // 1. Toggles
          // 2. Uses localStorage
          // Once we have a user facing widget to do this, it can all go
          if (item.startsWith('CONSUL_COLOR_SCHEME=')) {
            const [, value] = item.split('=');
            let current;
            try {
              current = JSON.parse(win.localStorage.getItem('consul:theme'));
            } catch (e) {
              current = {
                'color-scheme': 'light',
              };
            }
            win.localStorage.setItem(
              'consul:theme',
              `{"color-scheme": "${
                value === '!' ? (current['color-scheme'] === 'light' ? 'dark' : 'light') : value
              }"}`
            );
          } else {
            doc.cookie = `${item};Path=/`;
          }
        });
        win.location.hash = '';
        location.reload();
      } else {
        str = cookies(doc.cookie).join(';');
        const tab = win.open('', '_blank');
        tab.document.write(
          `<body><pre>${location.href}#${str}</pre><br /><a href="javascript:Scenario('${str}')">Scenario</a></body>`
        );
      }
    };

    if (
      typeof win.location !== 'undefined' &&
      typeof win.location.hash === 'string' &&
      win.location.hash.length > 0
    ) {
      win['Scenario'](win.location.hash.substr(1));
    }
  });

  // Defines a function that reads in the cookies and parses the cookie keys.
  const dev = function (str = doc.cookie) {
    return str
      .split(';')
      .filter((item) => item !== '')
      .map((item) => {
        const [key, ...rest] = item.trim().split('=');
        return [key, rest.join('=')];
      });
  };

  const user = function (str) {
    const item = win.localStorage.getItem(str);
    return item === null ? undefined : item;
  };
  const getResourceFor = function (src) {
    try {
      return (
        win.performance.getEntriesByType('resource').find((item) => {
          return item.initiatorType === 'script' && src === item.name;
        }) || {}
      );
    } catch (e) {
      return {};
    }
  };
  const operatorConfig = {
    ...config.operatorConfig,
    ...JSON.parse(doc.querySelector(`[data-${config.modulePrefix}-config]`).textContent),
  };
  const ui_config = operatorConfig.UIConfig || {};
  const scripts = doc.getElementsByTagName('script');
  // we use the currently executing script as a reference
  // to figure out where we are for other things such as
  // base url, api url etc
  const currentSrc = scripts[scripts.length - 1].src;
  let resource;
  // TODO: Potentially use ui_config {}, for example
  // turning off blocking queries if its a busy cluster
  // forcing/providing amount of possible HTTP connections
  // re-setting the base url for the API etc
  const operator = function (str, env) {
    let protocol, dashboards, provider, proxy;
    switch (str) {
      case 'CONSUL_NSPACES_ENABLED':
        return typeof operatorConfig.NamespacesEnabled === 'undefined'
          ? false
          : operatorConfig.NamespacesEnabled;
      case 'CONSUL_SSO_ENABLED':
        return typeof operatorConfig.SSOEnabled === 'undefined' ? false : operatorConfig.SSOEnabled;
      case 'CONSUL_ACLS_ENABLED':
        return typeof operatorConfig.ACLsEnabled === 'undefined'
          ? false
          : operatorConfig.ACLsEnabled;
      case 'CONSUL_PARTITIONS_ENABLED':
        return typeof operatorConfig.PartitionsEnabled === 'undefined'
          ? false
          : operatorConfig.PartitionsEnabled;
      case 'CONSUL_PEERINGS_ENABLED':
        return typeof operatorConfig.PeeringEnabled === 'undefined'
          ? false
          : operatorConfig.PeeringEnabled;
      case 'CONSUL_HCP_ENABLED':
        return typeof operatorConfig.HCPEnabled === 'undefined' ? false : operatorConfig.HCPEnabled;
      case 'CONSUL_DATACENTER_LOCAL':
        return operatorConfig.LocalDatacenter;
      case 'CONSUL_DATACENTER_PRIMARY':
        return operatorConfig.PrimaryDatacenter;
      case 'CONSUL_HCP_MANAGED_RUNTIME':
        return operatorConfig.HCPManagedRuntime;
      case 'CONSUL_API_PREFIX':
        // we want API prefix to look like an env var for if we ever change
        // operator config to be an API request, we need this variable before we
        // make and API request so this specific variable should never be be
        // retrived via an API request
        return operatorConfig.APIPrefix;
      case 'CONSUL_HCP_URL':
        return operatorConfig.HCPURL;
      case 'CONSUL_UI_CONFIG':
        dashboards = {
          service: undefined,
        };
        provider = env('CONSUL_METRICS_PROVIDER');
        proxy = env('CONSUL_METRICS_PROXY_ENABLED');
        dashboards.service = env('CONSUL_SERVICE_DASHBOARD_URL');
        if (provider) {
          ui_config.metrics_provider = provider;
        }
        if (proxy) {
          ui_config.metrics_proxy_enabled = proxy;
        }
        if (dashboards.service) {
          ui_config.dashboard_url_templates = dashboards;
        }
        return ui_config;
      case 'CONSUL_BASE_UI_URL':
        return currentSrc.split('/').slice(0, -2).join('/');
      case 'CONSUL_HTTP_PROTOCOL':
        if (typeof resource === 'undefined') {
          // resource needs to be retrieved lazily as entries aren't guaranteed
          // to be available at script execution time (caching seems to affect this)
          // waiting until we retrieve this value lazily at runtime means that
          // the entries are always available as these values are only retrieved
          // after initialization
          // current is based on the assumption that whereever this script is it's
          // likely to be the same as the xmlhttprequests
          resource = getResourceFor(currentSrc);
        }
        return resource.nextHopProtocol || 'http/1.1';
      case 'CONSUL_HTTP_MAX_CONNECTIONS':
        protocol = env('CONSUL_HTTP_PROTOCOL');
        // http/2, http2+QUIC/39 and SPDY don't have connection limits
        switch (true) {
          case protocol.indexOf('h2') === 0:
          case protocol.indexOf('hq') === 0:
          case protocol.indexOf('spdy') === 0:
            // TODO: Change this to return -1 so we try to consistently
            // return a value from env vars
            return;
          default:
            // generally 6 are available
            // reserve 1 for traffic that we can't manage
            return 5;
        }
    }
  };
  const ui = function (key) {
    let $ = {};
    switch (config.environment) {
      case 'development':
      case 'staging':
      case 'coverage':
      case 'test':
        $ = dev().reduce(function (prev, [key, value]) {
          switch (key) {
            case 'CONSUL_INTL_LOCALE':
              prev['CONSUL_INTL_LOCALE'] = String(value).toLowerCase();
              break;
            case 'CONSUL_INTL_DEBUG':
              prev['CONSUL_INTL_DEBUG'] = !!JSON.parse(String(value).toLowerCase());
              break;
            case 'CONSUL_ACLS_ENABLE':
              prev['CONSUL_ACLS_ENABLED'] = !!JSON.parse(String(value).toLowerCase());
              break;
            case 'CONSUL_AGENTLESS_ENABLE':
              prev['CONSUL_AGENTLESS_ENABLED'] = !!JSON.parse(String(value).toLowerCase());
              break;
            case 'CONSUL_NSPACES_ENABLE':
              prev['CONSUL_NSPACES_ENABLED'] = !!JSON.parse(String(value).toLowerCase());
              break;
            case 'CONSUL_SSO_ENABLE':
              prev['CONSUL_SSO_ENABLED'] = !!JSON.parse(String(value).toLowerCase());
              break;
            case 'CONSUL_PARTITIONS_ENABLE':
              prev['CONSUL_PARTITIONS_ENABLED'] = !!JSON.parse(String(value).toLowerCase());
              break;
            case 'CONSUL_METRICS_PROXY_ENABLE':
              prev['CONSUL_METRICS_PROXY_ENABLED'] = !!JSON.parse(String(value).toLowerCase());
              break;
            case 'CONSUL_PEERINGS_ENABLE':
              prev['CONSUL_PEERINGS_ENABLED'] = !!JSON.parse(String(value).toLowerCase());
              break;
            case 'CONSUL_HCP_ENABLE':
              prev['CONSUL_HCP_ENABLED'] = !!JSON.parse(String(value).toLowerCase());
              break;
            case 'CONSUL_UI_CONFIG':
              prev['CONSUL_UI_CONFIG'] = JSON.parse(value);
              break;
            case 'TokenSecretID':
              prev['CONSUL_HTTP_TOKEN'] = value;
              break;
            default:
              prev[key] = value;
          }
          return prev;
        }, {});
        break;
      case 'production':
        $ = dev().reduce(function (prev, [key, value]) {
          switch (key) {
            case 'TokenSecretID':
              prev['CONSUL_HTTP_TOKEN'] = value;
              break;
          }
          return prev;
        }, {});
        break;
    }
    if (typeof $[key] !== 'undefined') {
      return $[key];
    }
    return config[key];
  };
  return function env(str) {
    switch (str) {
      // All user providable values should start with CONSUL_UI
      // We allow the user to set these ones via localStorage
      // user value is preferred.
      case 'CONSUL_UI_DISABLE_REALTIME':
      case 'CONSUL_UI_DISABLE_ANCHOR_SELECTION':
        // these are booleans cast things out
        return !!JSON.parse(String(user(str) || 0).toLowerCase()) || ui(str);
      case 'CONSUL_UI_REALTIME_RUNNER':
        // these are strings
        return user(str) || ui(str);
      case 'CONSUL_UI_CONFIG':
      case 'CONSUL_DATACENTER_LOCAL':
      case 'CONSUL_DATACENTER_PRIMARY':
      case 'CONSUL_HCP_MANAGED_RUNTIME':
      case 'CONSUL_API_PREFIX':
      case 'CONSUL_HCP_URL':
      case 'CONSUL_ACLS_ENABLED':
      case 'CONSUL_NSPACES_ENABLED':
      case 'CONSUL_PEERINGS_ENABLED':
      case 'CONSUL_AGENTLESS_ENABLED':
      case 'CONSUL_HCP_ENABLED':
      case 'CONSUL_SSO_ENABLED':
      case 'CONSUL_PARTITIONS_ENABLED':
      case 'CONSUL_METRICS_PROVIDER':
      case 'CONSUL_METRICS_PROXY_ENABLE':
      case 'CONSUL_SERVICE_DASHBOARD_URL':
      case 'CONSUL_BASE_UI_URL':
      case 'CONSUL_HTTP_PROTOCOL':
      case 'CONSUL_HTTP_MAX_CONNECTIONS': {
        // We allow the operator to set these ones via various methods
        // although UI developer config is preferred
        const _ui = ui(str);
        return typeof _ui !== 'undefined' ? _ui : operator(str, env);
      }
      default:
        return ui(str);
    }
  };
}
