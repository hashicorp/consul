import Adapter from 'ember-data/adapters/rest';
import { inject as service } from '@ember/service';

import URL from 'url';
import createURL from 'consul-ui/utils/createURL';

export const REQUEST_CREATE = 'createRecord';
export const REQUEST_READ = 'queryRecord';
export const REQUEST_UPDATE = 'updateRecord';
export const REQUEST_DELETE = 'deleteRecord';
// export const REQUEST_READ_MULTIPLE = 'query';

export const DATACENTER_KEY = 'dc';

export default Adapter.extend({
  namespace: 'v1',
  repo: service('settings'),
  headersForRequest: function(params) {
    return {
      ...this.get('repo').findHeaders(),
      ...this._super(...arguments),
    };
  },
  cleanQuery: function(_query) {
    delete _query.id;
    const query = { ..._query };
    delete _query[DATACENTER_KEY];
    return query;
  },
  isQueryRecord: function(url) {
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
  slugFromURL: function(url) {
    // follow the 'last part of the url is the id' rule
    return decodeURIComponent(url.pathname.split('/').pop());
  },
  parseURL: function(str) {
    return new URL(str, this.getHost());
  },
  uidForURL: function(url, _slug = '') {
    const dc = url.searchParams.get(DATACENTER_KEY) || '';
    const slug = _slug === '' ? this.slugFromURL(url) : _slug;
    if (dc.length < 1) {
      throw new Error('Unable to create unique id, missing datacenter');
    }
    if (slug.length < 1) {
      throw new Error('Unable to create unique id, missing slug');
    }
    // TODO: we could use a URL here? They are unique AND useful
    // but probably slower to create?
    return JSON.stringify([dc, slug]);
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
