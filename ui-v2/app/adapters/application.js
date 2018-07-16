import Adapter from 'ember-data/adapters/rest';
import { inject as service } from '@ember/service';

import URL from 'url';
import createURL from 'consul-ui/utils/createURL';
import { FOREIGN_KEY as DATACENTER_KEY } from 'consul-ui/models/dc';

export const REQUEST_CREATE = 'createRecord';
export const REQUEST_READ = 'queryRecord';
export const REQUEST_UPDATE = 'updateRecord';
export const REQUEST_DELETE = 'deleteRecord';
// export const REQUEST_READ_MULTIPLE = 'query';

export const DATACENTER_QUERY_PARAM = 'dc';

export default Adapter.extend({
  namespace: 'v1',
  repo: service('settings'),
  headersForRequest: function(params) {
    return {
      ...this.get('repo').findHeaders(),
      ...this._super(...arguments),
    };
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
    delete _query.id;
    const query = { ..._query };
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
    return createURL([this.buildURL(), path], parts, query);
  },
});
