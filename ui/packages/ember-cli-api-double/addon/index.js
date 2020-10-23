import Pretender from 'pretender';
import apiFactory from 'api-double';
import htmlReader from 'api-double/reader/html';
import deepAssign from 'merge-options';

const defaultGetCookiesFor = function(cookies = document.cookie) {
  return function(type, value, obj = {}) {
    return cookies.split(';').reduce(
      function(prev, item) {
        const temp = item.split('=');
        prev[temp[0].trim()] = temp[1];
        return prev;
      },
      {}
    )
  }
}
const defaultGetShouldMutateCallback = function() {
  return function() {
    return function(type) {
      return function() {
        return false;
      }
    }
  }
}
export default function(config = {}, getCookiesFor = defaultGetCookiesFor(), getShouldMutateCallback = defaultGetShouldMutateCallback()) {
  const key = Object.keys(config.endpoints)[0];
  const path = config.endpoints[key].replace(key, '');
  const salt = typeof config.salt === 'undefined' ? 12345 : config.salt;
  const reader = typeof config.reader === 'undefined' ? 'html' : config.reader;
  let createAPI;
  if(reader === 'html') {
    createAPI = apiFactory(salt, '', htmlReader);
  } else {
    createAPI = apiFactory(salt, path);
  }
  let api = createAPI();
  let cookies = {};
  let history = [];
  let statuses = {};
  let bodies = {};
  const server = new Pretender();
  server.handleRequest = function(request) {
    const found = Object.keys(config.endpoints).find(
      function(url) {
        return request.url.startsWith(url);
      }
    );
    if(!found) {
      request.passthrough();
    } else {
      const temp = request.url.split('?');
      let url = temp[0];
      let queryParams = {};
      if(temp[1]) {
        queryParams = temp[1].split('&').reduce(
          function(prev, item) {
            const temp = item.split('=');
            prev[decodeURIComponent(temp[0])] = decodeURIComponent(temp[1]);
            return prev;
          },
          queryParams
        );
      }
      history.push(request);
      const req = {
        path: url,
        url: url,
        query: queryParams,
        headers: request.requestHeaders,
        body: request.requestBody,
        method: request.method,
        cookies: Object.assign(cookies, getCookiesFor('*'))
      };
      let headers = { 'Content-Type': 'application/json' };
      const response = {
        _status: 200,
        set: function(_headers) {
          headers = Object.assign({}, headers, _headers);
        },
        send: function(response) {
          request.respond(statuses[url] || this._status, headers, bodies[url] || response);
        },
        status: function(status) {
          this._status = status;
          return this;
        }
      };
      api.serve(req, response, function() {});

    }
  };
  return {
    api: api,
    server: {
      history: history,
      clearHistory: function() {
        history = [];
        this.history = history;
      },
      clearResponses: function() {
        statuses = {};
        bodies = {};
      },
      clearCookies: function() {
        cookies = {};
      },
      reset: function() {
        api = createAPI();
        this.clearCookies();
        this.clearResponses();
        this.clearHistory();
      },
      setCookie: function(name, value) {
        cookies[name] = value;
      },
      respondWithStatus: function(url, s) {
        statuses[url] = s;
      },
      respondWith: function(url, response) {
        statuses[url] = response.status || 200;
        bodies[url] = response.body || '';
      },
      // keep mirage-like interface
      createList: function(type, num, value) {
        cookies = Object.assign(
          cookies,
          getCookiesFor(type, num)
        );

        if (typeof value !== 'undefined') {
          api.mutate(
            function(response, config) {
              if (typeof response.map !== 'function') {
                try {
                  return deepAssign(response, value)
                } catch(e) {
                  // unable to merge the objects
                  return response;
                }
              }
              return response.map((item, i, arr) => {
                let res = value;
                if (typeof value === 'object') {
                  if (value.constructor == Object) {
                    // res = { ...item, ...value };
                    if(typeof item === 'string') {
                      res = value.toString();
                    } else {
                      res = deepAssign(item, value);
                    }
                  } else if (value.constructor == Array) {
                    // res = { ...item, ...value[i] };
                    if(value[i]) {
                      if(typeof value[i] === "object") {
                        res = deepAssign(item, value[i]);
                      } else {
                        res = value[i];
                      }
                    }
                  }
                }
                return res;
              });
            },
            getShouldMutateCallback(type)
          );
        }
      },
    },
  };

}
