import _config from './config/environment';
const doc = document;
const getDevEnvVars = function() {
  return doc.cookie.split(';').map(item => item.trim().split('='));
};
const getUserEnvVar = function(str) {
  return window.localStorage.getItem(str);
};
// TODO: Look at `services/client` for pulling
// HTTP headers in here so we can let things be controlled
// via HTTP proxies, for example turning off blocking
// queries if its a busy cluster
// const getOperatorEnvVars = function() {}

// TODO: Not necessarily here but the entire app should
// use the `env` export not the `default` one
// but we might also change the name of this file, so wait for that first
export const env = function(str) {
  let user = null;
  switch (str) {
    case 'CONSUL_UI_DISABLE_REALTIME':
    case 'CONSUL_UI_DISABLE_ANCHOR_SELECTION':
    case 'CONSUL_UI_REALTIME_RUNNER':
      user = getUserEnvVar(str);
      break;
  }
  // We use null here instead of an undefined check
  // as localStorage will return null if not set
  return user !== null ? user : _config[str];
};
export const config = function(key) {
  switch (_config.environment) {
    case 'development':
    case 'staging':
    case 'testing':
      const $ = getDevEnvVars().reduce(function(prev, [key, value]) {
        const val = !!JSON.parse(String(value).toLowerCase());
        switch (key) {
          case 'CONSUL_ACLS_ENABLE':
            prev['CONSUL_ACLS_ENABLED'] = val;
            break;
          case 'CONSUL_NSPACES_ENABLE':
            prev['CONSUL_NSPACES_ENABLED'] = val;
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
  return _config[key];
};
export default env;
