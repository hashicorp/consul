import { inject as service } from '@ember/service';
import { set, action } from '@ember/object';

let popstateFired = false;

const _uuid = function() {
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, c => {
    const r = (Math.random() * 16) | 0;
    return (c === 'x' ? r : (r & 3) | 8).toString(16);
  });
};

export default class {
  @service('-document') doc;
  @service('env') env;

  implementation = 'regexp';

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

  constructor(owner) {
    this.container = Object.entries(owner)[0][1];
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
    this.history = this.history || this.doc.defaultView.history;
    const state = this.history.state;
    this.doc.defaultView.addEventListener('popstate', this.route);
    // let path = this.location.pathname;//
    const path = this.formatURL(this.getURLForTransition(this.location.pathname));
    console.log(path, state.path, 'initState');
    if (state && state.path === path) {
      // preserve existing state
      // used for webkit workaround, since there will be no initial popstate event
      this._previousPath = path;
      this._previousURL = this.getURL();
    } else {
      console.log('hree');
      this.dispatch('replace', path);
    }
  }

  optionalParams() {
    let optional = Object.entries(this.optional || {});
    return optional.reduce((prev, [key, value]) => {
      prev[key] = value.match;
      return prev;
    }, {});
  }

  hrefTo(routeName, params, hash) {
    if (typeof hash.dc !== 'undefined') {
      delete hash.dc;
    }
    if (typeof hash.nspace !== 'undefined') {
      hash.nspace = `~${hash.nspace}`;
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

  getURLFrom(url) {
    // remove trailing slashes if they exists
    url = url || this.location.pathname;
    this.rootURL = this.rootURL.replace(/\/$/, '');
    this.baseURL = this.baseURL.replace(/\/$/, '');

    // remove baseURL and rootURL from start of path
    return url
      .replace(new RegExp(`^${this.baseURL}(?=/|$)`), '')
      .replace(new RegExp(`^${this.rootURL}(?=/|$)`), '')
      .replace(/\/\//g, '/'); // remove extra slashes
  }

  getOptionalParamsFromURL() {}

  getURLForTransition(url) {
    const optional = {};
    if (this.env.var('CONSUL_NSPACES_ENABLED')) {
      optional.nspace = /^~([a-zA-Z0-9]([a-zA-Z0-9-]{0,62}[a-zA-Z0-9])?)$/;
    }
    this.optional = {};
    url = this.getURLFrom(url)
      .split('/')
      .filter((item, i) => {
        if (i < 3) {
          let found = false;
          Object.entries(optional).reduce((prev, [key, re]) => {
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
  /**
   * Takes a full browser URL including rootURL and optional and performs an
   * ember transition/refresh and browser location update using that
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

  formatURL(url, optional, withOptional = true) {
    if (url !== '') {
      // remove trailing slashes if they exists
      this.rootURL = this.rootURL.replace(/\/$/, '');
      this.baseURL = this.baseURL.replace(/\/$/, '');
    } else if (this.baseURL[0] === '/' && this.rootURL[0] === '/') {
      // if baseURL and rootURL both start with a slash
      // ... remove trailing slash from baseURL if it exists
      this.baseURL = this.baseURL.replace(/\/$/, '');
    }
    if (withOptional) {
      const temp = url.split('/');
      if (Object.keys(optional || {}).length === 0) {
        optional = undefined;
      }
      optional = Object.entries(optional || this.optional || {});
      optional = optional.reduce((prev, [key, value]) => value.value || value, []);
      temp.splice(...[1, 0].concat(optional));
      url = temp.join('/');
    }

    return `${this.baseURL}${this.rootURL}${url}`;
  }
  /**
   * Change URL takes an ember application URL
   */
  changeURL(type, path) {
    this.path = path; //this.getURLForTransition(path);
    console.log(path, this.path, `${type}URL`, this.optional);
    const state = this.history.state;
    path = this.formatURL(path);

    if (!state || state.path !== path) {
      this.dispatch(type, path);
    }
  }

  setURL(path) {
    // this.optional = {};
    console.log(path, 'setURL');
    this.changeURL('push', path);
  }

  replaceURL(path) {
    console.log(path, 'replaceURL');
    this.changeURL('replace', path);
  }
  /**
   * Dispatch takes a full actual browser URL with all the rootURL and optional
   * params if they exist
   */
  dispatch(event, path) {
    const state = { path, uuid: _uuid() };
    console.log('dispatch', path);
    this.history[`${event}State`](state, null, path);
    // popstate listeners only run from a browser action not when a state change
    // is called directly, so manually call the popstate listener.
    // https://developer.mozilla.org/en-US/docs/Web/API/Window/popstate_event#the_history_stack
    this.route({ state: state });
  }

  /**
   * Register a callback to be invoked whenever the browser history changes,
   * including using forward and back buttons.
   */
  @action
  route(e) {
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
      this.container.lookup('route:application').refresh();
      return;
    }
    if (this.callback) {
      this.callback(url);
    }
    // used for webkit workaround
    this._previousURL = url;
    this._previousPath = e.state.path;
  }
  onUpdateURL(callback) {
    this.callback = callback;
  }

  willDestroy() {
    this.doc.defaultView.removeEventListener('popstate', this.route);
  }
}
