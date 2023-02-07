import config from 'consul-ui/config/environment';

import apiDouble from '@hashicorp/ember-cli-api-double';
import setCookies from 'consul-ui/tests/helpers/set-cookies';
import typeToURL from 'consul-ui/tests/helpers/type-to-url';

const addon = config['@hashicorp/ember-cli-api-double'];
const api = apiDouble(addon, setCookies, typeToURL);
export const get = function (_url, options = { headers: { cookie: {} } }) {
  const url = new URL(_url, 'http://localhost');
  return new Promise(function (resolve) {
    return api.api.serve(
      {
        method: 'GET',
        path: url.pathname,
        url: url.href,
        cookies: options.headers.cookie || {},
        headers: {},
        query: [...url.searchParams.keys()].reduce(function (prev, key) {
          prev[key] = url.searchParams.get(key);
          return prev;
        }, {}),
      },
      {
        set: function () {},
        status: function () {
          return this;
        },
        send: function (content) {
          resolve(JSON.parse(content));
        },
      },
      function () {}
    );
  });
};
export default api;
