import Adapter from 'ember-data/adapters/rest';
import { AbortError } from 'ember-data/adapters/errors';
import { inject as service } from '@ember/service';
import { get } from '@ember/object';

import URL from 'url';
import createURL from 'consul-ui/utils/createURL';
import { FOREIGN_KEY as DATACENTER_KEY } from 'consul-ui/models/dc';
import { HEADERS_SYMBOL as HTTP_HEADERS_SYMBOL } from 'consul-ui/utils/http/consul';

export const REQUEST_CREATE = 'createRecord';
export const REQUEST_READ = 'queryRecord';
export const REQUEST_UPDATE = 'updateRecord';
export const REQUEST_DELETE = 'deleteRecord';
// export const REQUEST_READ_MULTIPLE = 'query';

export const DATACENTER_QUERY_PARAM = 'dc';

export default Adapter.extend({
  namespace: 'v1',
  repo: service('settings'),
  client: service('client/http'),
  manageConnection: function(options) {
    const client = get(this, 'client');
    const complete = options.complete;
    const beforeSend = options.beforeSend;
    options.beforeSend = function(xhr) {
      if (typeof beforeSend === 'function') {
        beforeSend(...arguments);
      }
      options.id = client.request(options, xhr);
    };
    options.complete = function(xhr, textStatus) {
      client.complete(options.id);
      if (typeof complete === 'function') {
        complete(...arguments);
      }
    };
    return options;
  },
  _ajaxRequest: function(options) {
    return this._super(this.manageConnection(options));
  },
  queryRecord: function() {
    return this._super(...arguments).catch(function(e) {
      if (e instanceof AbortError) {
        e.errors[0].status = '0';
      }
      throw e;
    });
  },
  query: function() {
    return this._super(...arguments).catch(function(e) {
      if (e instanceof AbortError) {
        e.errors[0].status = '0';
      }
      throw e;
    });
  },
  headersForRequest: function(params) {
    return {
      ...this.get('repo').findHeaders(),
      ...this._super(...arguments),
    };
  },
  handleResponse: function(status, headers, response, requestData) {
    // The ember-data RESTAdapter drops the headers after this call,
    // and there is no where else to get to these
    // save them to response[HTTP_HEADERS_SYMBOL] for the moment
    // so we can save them as meta in the serializer...
    if (
      (typeof response == 'object' && response.constructor == Object) ||
      Array.isArray(response)
    ) {
      // lowercase everything incase we get browser inconsistencies
      const lower = {};
      Object.keys(headers).forEach(function(key) {
        lower[key.toLowerCase()] = headers[key];
      });
      response[HTTP_HEADERS_SYMBOL] = lower;
    }
    return this._super(status, headers, response, requestData);
  },
  handleBooleanResponse: function(url, response, primary, slug) {
    return {
      // consider a check for a boolean, also for future me,
      // response[slug] // this will forever be null, response should be boolean
      [primary]: this.uidForURL(url /* response[slug]*/),
    };
  },
  // could always consider an extra 'dc' arg on the end here?
  handleSingleResponse: function(url, response, primary, slug, _dc) {
    const dc =
      typeof _dc !== 'undefined' ? _dc : url.searchParams.get(DATACENTER_QUERY_PARAM) || '';
    return {
      ...response,
      ...{
        [DATACENTER_KEY]: dc,
        [primary]: this.uidForURL(url, response[slug]),
      },
    };
  },
  handleBatchResponse: function(url, response, primary, slug) {
    const dc = url.searchParams.get(DATACENTER_QUERY_PARAM) || '';
    return response.map((item, i, arr) => {
      return this.handleSingleResponse(url, item, primary, slug, dc);
    });
  },
  cleanQuery: function(_query) {
    if (typeof _query.id !== 'undefined') {
      delete _query.id;
    }
    const query = { ..._query };
    if (typeof query.separator !== 'undefined') {
      delete query.separator;
    }
    if (typeof query.index !== 'undefined') {
      delete query.index;
    }
    delete _query[DATACENTER_QUERY_PARAM];
    return query;
  },
  isUpdateRecord: function(url, method) {
    return false;
  },
  isCreateRecord: function(url, method) {
    return false;
  },
  isQueryRecord: function(url, method) {
    // this is ONLY if ALL api's using it
    // follow the 'last part of the url is the id' rule
    const pathname = url.pathname
      .split('/') // unslashify
      // remove the last
      .slice(0, -1)
      // add and empty to ensure a trailing slash
      .concat([''])
      // slashify
      .join('/');
    // compare with empty id against empty id
    return pathname === this.parseURL(this.urlForQueryRecord({ id: '' })).pathname;
  },
  getHost: function() {
    return this.host || `${location.protocol}//${location.host}`;
  },
  slugFromURL: function(url, decode = decodeURIComponent) {
    // follow the 'last part of the url is the id' rule
    return decode(url.pathname.split('/').pop());
  },
  parseURL: function(str) {
    return new URL(str, this.getHost());
  },
  uidForURL: function(url, _slug = '', hash = JSON.stringify) {
    const dc = url.searchParams.get(DATACENTER_QUERY_PARAM) || '';
    const slug = _slug === '' ? this.slugFromURL(url) : _slug;
    if (dc.length < 1) {
      throw new Error('Unable to create unique id, missing datacenter');
    }
    if (slug.length < 1) {
      throw new Error('Unable to create unique id, missing slug');
    }
    // TODO: we could use a URL here? They are unique AND useful
    // but probably slower to create?
    return hash([dc, slug]);
  },

  // appendURL in turn calls createURL
  // createURL ensures that all `parts` are URL encoded
  // and all `query` values are URL encoded

  // `this.buildURL()` with no arguments will give us `${host}/${namespace}`
  // `path` is the user configurable 'urlsafe' string to append on `buildURL`
  // `parts` is an array of possibly non 'urlsafe parts' to be encoded and
  // appended onto the url
  // `query` will populate the query string. Again the values of which will be
  // url encoded

  appendURL: function(path, parts = [], query = {}) {
    // path can be a string or an array of parts that will be slash joined
    return createURL([this.buildURL()].concat(path), parts, query);
  },
});
