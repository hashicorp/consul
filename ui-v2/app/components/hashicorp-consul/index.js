import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { get, set, computed } from '@ember/object';
import { getOwner } from '@ember/application';

export default Component.extend({
  dom: service('dom'),
  env: service('env'),
  feedback: service('feedback'),
  router: service('router'),
  http: service('repository/type/event-source'),
  client: service('client/http'),
  store: service('store'),
  settings: service('settings'),

  didInsertElement: function() {
    this.dom.root().classList.remove('template-with-vertical-menu');
  },
  // TODO: Right now this is the only place where we need permissions
  // but we are likely to need it elsewhere, so probably need a nice helper
  canManageNspaces: computed('permissions', function() {
    return (
      typeof (this.permissions || []).find(function(item) {
        return item.Resource === 'operator' && item.Access === 'write' && item.Allow;
      }) !== 'undefined'
    );
  }),
  forwardForACL: function(token) {
    let routeName = this.router.currentRouteName;
    const route = getOwner(this).lookup(`route:${routeName}`);
    // a null AccessorID means we are in legacy mode
    // take the user to the legacy acls
    // otherwise just refresh the page
    if (get(token, 'AccessorID') === null) {
      // returning false for a feedback action means even though
      // its successful, please skip this notification and don't display it
      return route.transitionTo('dc.acls');
    } else {
      // TODO: Ideally we wouldn't need to use env() at a component level
      // transitionTo should probably remove it instead if NSPACES aren't enabled
      if (this.env.var('CONSUL_NSPACES_ENABLED') && get(token, 'Namespace') !== this.nspace.Name) {
        if (!routeName.startsWith('nspace')) {
          routeName = `nspace.${routeName}`;
        }
        const nspace = get(token, 'Namespace');
        // you potentially have a new namespace
        if (typeof nspace !== 'undefined') {
          return route.transitionTo(`${routeName}`, `~${nspace}`, this.dc.Name);
        }
        // you are logging out, just refresh
        return route.refresh();
      } else {
        if (route.routeName === 'dc.acls.index') {
          return route.transitionTo('dc.acls.tokens.index');
        }
        return route.refresh();
      }
    }
  },
  actions: {
    send: function(el, method, ...rest) {
      const component = this.dom.component(el);
      component.actions[method].apply(component, rest || []);
    },
    changeToken: function(token = {}) {
      const prev = this.token;
      if (token === '') {
        token = {};
      }
      set(this, 'token', token);
      // if this is just the initial 'find out what the current token is'
      // then don't do anything
      if (typeof prev === 'undefined') {
        return;
      }
      let notification;
      let action = () => this.forwardForACL(token);
      switch (true) {
        case get(this, 'token.AccessorID') === null && get(this, 'token.SecretID') === null:
          // 'everything is null, 403 this needs deleting' token
          this.settings.delete('token');
          return;
        case get(prev, 'AccessorID') === null && get(prev, 'SecretID') === null:
          // we just had an 'everything is null, this needs deleting' token
          // reject and break so this acts differently to just logging out
          action = () => Promise.reject({});
          notification = 'authorize';
          break;
        case typeof get(prev, 'AccessorID') !== 'undefined' &&
          typeof get(this, 'token.AccessorID') !== 'undefined':
          // change of both Accessor and Secret, means use
          notification = 'use';
          break;
        case get(this, 'token.AccessorID') === null &&
          typeof get(this, 'token.SecretID') !== 'undefined':
          // legacy login, don't do anything as we don't use self for auth here but the endpoint itself
          // self is successful, but skip this notification and don't display it
          return this.forwardForACL(token);
        case typeof get(prev, 'AccessorID') === 'undefined' &&
          typeof get(this, 'token.AccessorID') !== 'undefined':
          // normal login
          notification = 'authorize';
          break;
        case (typeof get(prev, 'AccessorID') !== 'undefined' || get(prev, 'AccessorID') === null) &&
          typeof get(this, 'token.AccessorID') === 'undefined':
          //normal logout
          notification = 'logout';
          break;
      }
      this.actions.reauthorize.apply(this, [
        {
          type: notification,
          action: action,
        },
      ]);
    },
    reauthorize: function(e) {
      this.client.abort();
      this.http.resetCache();
      this.store.init();
      const type = get(e, 'type');
      this.feedback.execute(
        e.action,
        type,
        function(type, e) {
          return type;
        },
        {}
      );
    },
    change: function(e) {
      const win = this.dom.viewport();
      const $root = this.dom.root();
      const $body = this.dom.element('body');
      if (e.target.checked) {
        $root.classList.add('template-with-vertical-menu');
        $body.style.height = $root.style.height = win.innerHeight + 'px';
      } else {
        $root.classList.remove('template-with-vertical-menu');
        $body.style.height = $root.style.height = null;
      }
    },
  },
});
