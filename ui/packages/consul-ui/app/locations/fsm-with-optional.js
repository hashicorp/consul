import { env } from 'consul-ui/env';
const OPTIONAL = {};
if (env('CONSUL_PARTITIONS_ENABLED')) {
  OPTIONAL.partition = /^-([a-zA-Z0-9]([a-zA-Z0-9-]{0,62}[a-zA-Z0-9])?)$/;
}

if (env('CONSUL_NSPACES_ENABLED')) {
  OPTIONAL.nspace = /^~([a-zA-Z0-9]([a-zA-Z0-9-]{0,62}[a-zA-Z0-9])?)$/;
}

const trailingSlashRe = /\/$/;

// see below re: ember double slashes
// const moreThan1SlashRe = /\/{2,}/g;

const _uuid = function() {
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, c => {
    const r = (Math.random() * 16) | 0;
    return (c === 'x' ? r : (r & 3) | 8).toString(16);
  });
};

// let popstateFired = false;
/**
 * Register a callback to be invoked whenever the browser history changes,
 * including using forward and back buttons.
 */
const route = function(e) {
  const path = e.state.path;
  const url = this.getURLForTransition(path);
  // Ignore initial page load popstate event in Chrome
  // if (!popstateFired) {
  //   popstateFired = true;
  //   if (url === this._previousURL) {
  //     return;
  //   }
  // }
  if (url === this._previousURL) {
    if (path === this._previousPath) {
      return;
    }
    this._previousPath = e.state.path;
    // async
    this.container.lookup('route:application').refresh();
  }
  if (typeof this.callback === 'function') {
    // TODO: Can we use `settled` or similar to make this `route` method async?
    // not async
    this.callback(url);
  }
  // used for webkit workaround
  this._previousURL = url;
  this._previousPath = e.state.path;
};
export default class FSMWithOptionalLocation {
  // extend FSMLocation
  implementation = 'fsm-with-optional';

  baseURL = '';
  /**
   * Set from router:main._setupLocation (-internals/routing/lib/system/router)
   * Will be pre-pended to path upon state change
   */
  rootURL = '/';

  /**
   * Path is the 'application path' i.e. the path/URL with no root/base URLs
   * but potentially with optional parameters (these are remove when getURL is called)
   */
  path = '/';

  /**
   * Sneaky undocumented property used in ember's main router used to skip any
   * setup of location from the main router. We currently don't need this but
   * document it here incase we ever do.
   */
  cancelRouterSetup = false;

  /**
   * Used to store our 'optional' segments should we have any
   */
  optional = {};

  static create() {
    return new this(...arguments);
  }

  constructor(owner, doc, env) {
    this.container = Object.entries(owner)[0][1];

    // add the route/state change handler
    this.route = route.bind(this);

    this.doc = typeof doc === 'undefined' ? this.container.lookup('service:-document') : doc;
    this.env = typeof env === 'undefined' ? this.container.lookup('service:env') : env;

    const base = this.doc.querySelector('base[href]');
    if (base !== null) {
      this.baseURL = base.getAttribute('href');
    }
  }

  /**
   * @internal
   * Called from router:main._setupLocation (-internals/routing/lib/system/router)
   * Used to set state on first call to setURL
   */
  initState() {
    this.location = this.location || this.doc.defaultView.location;
    this.machine = this.machine || this.doc.defaultView.history;
    this.doc.defaultView.addEventListener('popstate', this.route);

    const state = this.machine.state;
    const url = this.getURL();
    const href = this.formatURL(url);

    if (state && state.path === href) {
      // preserve existing state
      // used for webkit workaround, since there will be no initial popstate event
      this._previousPath = href;
      this._previousURL = url;
    } else {
      this.dispatch('replace', href);
    }
  }

  getURLFrom(url) {
    // remove trailing slashes if they exist
    url = url || this.location.pathname;
    this.rootURL = this.rootURL.replace(trailingSlashRe, '');
    this.baseURL = this.baseURL.replace(trailingSlashRe, '');
    // remove baseURL and rootURL from start of path
    return url
      .replace(new RegExp(`^${this.baseURL}(?=/|$)`), '')
      .replace(new RegExp(`^${this.rootURL}(?=/|$)`), '');
    // ember default locations remove double slashes here e.g. '//'
    // .replace(moreThan1SlashRe, '/'); // remove extra slashes
  }

  getURLForTransition(url) {
    this.optional = {};
    url = this.getURLFrom(url)
      .split('/')
      .filter((item, i) => {
        if (i < 3) {
          let found = false;
          Object.entries(OPTIONAL).reduce((prev, [key, re]) => {
            const res = re.exec(item);
            if (res !== null) {
              prev[key] = {
                value: item,
                match: res[1],
              };
              found = true;
            }
            return prev;
          }, this.optional);
          return !found;
        }
        return true;
      })
      .join('/');
    return url;
  }

  optionalParams() {
    let optional = this.optional || {};
    return ['partition', 'nspace'].reduce((prev, item) => {
      let value = '';
      if (typeof optional[item] !== 'undefined') {
        value = optional[item].match;
      }
      prev[item] = value;
      return prev;
    }, {});
  }

  // public entrypoints for app hrefs/URLs

  // visit and transitionTo can't be async/await as they return promise-like
  // non-promises that get re-wrapped by the addition of async/await
  visit() {
    return this.transitionTo(...arguments);
  }

  /**
   * Turns a routeName into a full URL string for anchor hrefs etc.
   */
  hrefTo(routeName, params, hash) {
    if (typeof hash.dc !== 'undefined') {
      delete hash.dc;
    }
    if (typeof hash.nspace !== 'undefined') {
      hash.nspace = `~${hash.nspace}`;
    }
    if (typeof hash.partition !== 'undefined') {
      hash.partition = `-${hash.partition}`;
    }
    if (typeof this.router === 'undefined') {
      this.router = this.container.lookup('router:main');
    }
    const router = this.router._routerMicrolib;
    const url = router.generate(routeName, ...params, {
      queryParams: {},
    });
    let withOptional = true;
    switch (true) {
      case routeName === 'settings':
      case routeName.startsWith('docs.'):
        withOptional = false;
    }
    return this.formatURL(url, hash, withOptional);
  }

  /**
   * Takes a full browser URL including rootURL and optional (a full href) and
   * performs an ember transition/refresh and browser location update using that
   */
  transitionTo(url) {
    const transitionURL = this.getURLForTransition(url);
    if (this._previousURL === transitionURL) {
      // probably an optional parameter change
      this.dispatch('push', url);
      return Promise.resolve();
      // this.setURL(url);
    } else {
      // use ember to transition, which will eventually come around to use location.setURL
      return this.container.lookup('router:main').transitionTo(transitionURL);
    }
  }

  //

  // Ember location interface

  /**
   * Returns the current `location.pathname` without `rootURL` or `baseURL`
   */
  getURL() {
    const search = this.location.search || '';
    let hash = '';
    if (typeof this.location.hash !== 'undefined') {
      hash = this.location.hash.substr(0);
    }
    const url = this.getURLForTransition(this.location.pathname);
    return `${url}${search}${hash}`;
  }

  formatURL(url, optional, withOptional = true) {
    if (url !== '') {
      // remove trailing slashes if they exists
      this.rootURL = this.rootURL.replace(trailingSlashRe, '');
      this.baseURL = this.baseURL.replace(trailingSlashRe, '');
    } else if (this.baseURL[0] === '/' && this.rootURL[0] === '/') {
      // if baseURL and rootURL both start with a slash
      // ... remove trailing slash from baseURL if it exists
      this.baseURL = this.baseURL.replace(trailingSlashRe, '');
    }

    if (withOptional) {
      const temp = url.split('/');
      if (Object.keys(optional || {}).length === 0) {
        optional = undefined;
      }
      optional = Object.values(optional || this.optional || {});
      optional = optional.filter(item => Boolean(item)).map(item => item.value || item, []);
      temp.splice(...[1, 0].concat(optional));
      url = temp.join('/');
    }

    return `${this.baseURL}${this.rootURL}${url}`;
  }
  /**
   * Change URL takes an ember application URL
   */
  changeURL(type, path) {
    this.path = path;
    const state = this.machine.state;
    path = this.formatURL(path);

    if (!state || state.path !== path) {
      this.dispatch(type, path);
    }
  }

  setURL(path) {
    // this.optional = {};
    this.changeURL('push', path);
  }

  replaceURL(path) {
    this.changeURL('replace', path);
  }

  onUpdateURL(callback) {
    this.callback = callback;
  }

  //

  /**
   * Dispatch takes a full actual browser URL with all the rootURL and optional
   * params if they exist
   */
  dispatch(event, path) {
    const state = {
      path: path,
      uuid: _uuid(),
    };
    this.machine[`${event}State`](state, null, path);
    // popstate listeners only run from a browser action not when a state change
    // is called directly, so manually call the popstate listener.
    // https://developer.mozilla.org/en-US/docs/Web/API/Window/popstate_event#the_history_stack
    this.route({ state: state });
  }

  willDestroy() {
    this.doc.defaultView.removeEventListener('popstate', this.route);
  }
}
