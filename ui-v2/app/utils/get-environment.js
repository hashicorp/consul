export default function(config = {}, win = window, doc = document) {
  const dev = function() {
    return doc.cookie
      .split(';')
      .filter(item => item !== '')
      .map(item => item.trim().split('='));
  };
  const user = function(str) {
    const item = win.localStorage.getItem(str);
    return item === null ? undefined : item;
  };
  const getResourceFor = function(src) {
    try {
      return (
        win.performance.getEntriesByType('resource').find(item => {
          return item.initiatorType === 'script' && src === item.name;
        }) || {}
      );
    } catch (e) {
      return {};
    }
  };
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
  const operator = function(str, env) {
    let protocol;
    switch (str) {
      case 'CONSUL_BASE_UI_URL':
        return currentSrc
          .split('/')
          .slice(0, -2)
          .join('/');
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
  const ui = function(key) {
    let $;
    switch (config.environment) {
      case 'development':
      case 'staging':
      case 'coverage':
      case 'test':
        $ = dev().reduce(function(prev, [key, value]) {
          switch (key) {
            case 'CONSUL_ACLS_ENABLE':
              prev['CONSUL_ACLS_ENABLED'] = !!JSON.parse(String(value).toLowerCase());
              break;
            case 'CONSUL_NSPACES_ENABLE':
              prev['CONSUL_NSPACES_ENABLED'] = !!JSON.parse(String(value).toLowerCase());
              break;
            case 'CONSUL_SSO_ENABLE':
              prev['CONSUL_SSO_ENABLED'] = !!JSON.parse(String(value).toLowerCase());
              break;
            default:
              prev[key] = value;
          }
          return prev;
        }, {});
        if (typeof $[key] !== 'undefined') {
          return $[key];
        }
        break;
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

      case 'CONSUL_BASE_UI_URL':
      case 'CONSUL_HTTP_PROTOCOL':
      case 'CONSUL_HTTP_MAX_CONNECTIONS':
        // We allow the operator to set these ones via various methods
        // although UI developer config is preferred
        return ui(str) || operator(str, env);
      default:
        return ui(str);
    }
  };
}
