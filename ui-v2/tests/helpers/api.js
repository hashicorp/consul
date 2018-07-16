import getAPI from '@hashicorp/ember-cli-api-double';
import setCookies from 'consul-ui/tests/helpers/set-cookies';
import typeToURL from 'consul-ui/tests/helpers/type-to-url';
import config from 'consul-ui/config/environment';
const apiConfig = config['ember-cli-api-double'];
let path = '/consul-api-double';
let reader;
if (apiConfig) {
  const temp = apiConfig.endpoints[0].split('/');
  reader = apiConfig.reader;
  temp.pop();
  path = temp.join('/');
}
const api = getAPI(path, setCookies, typeToURL, reader);
export const get = function(_url, options = { headers: { cookie: {} } }) {
  const url = new URL(_url, 'http://localhost');
  return new Promise(function(resolve) {
    return api.api.serve(
      {
        method: 'GET',
        path: url.pathname,
        url: url.href,
        cookies: options.headers.cookie || {},
        query: [...url.searchParams.keys()].reduce(function(prev, key) {
          prev[key] = url.searchParams.get(key);
          return prev;
        }, {}),
      },
      {
        send: function(content) {
          resolve(JSON.parse(content));
        },
      },
      function() {}
    );
  });
};
export default api;
