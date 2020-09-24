import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { setProperties } from '@ember/object';

class State {
  constructor(name) {
    this.name = name;
  }
  matches(match) {
    return this.name === match;
  }
}
const outlets = new Set();
export default Component.extend({
  tagName: '',
  router: service('router'),
  dom: service('dom'),
  root: false,
  init: function() {
    this._super(...arguments);
    this._listeners = this.dom.listeners();
    if (outlets.size === 0) {
      this.root = true;
    }
    if (this.root) {
      this.setAppRoute(this.router.currentRouteName);
      this.setAppState('idle');
    }
  },
  didDestroyElement: function() {
    this._super(...arguments);
    outlets.delete(this.name);
    this._listeners.remove();
  },
  setAppRoute: function(name) {
    if (name.startsWith('nspace.')) {
      name = name.substr(0, 'nspace.'.length);
    }
    this.dom.root().dataset.route = name;
  },
  setAppState: function(state) {
    this.dom.root().dataset.state = state;
  },
  didInsertElement: function() {
    this._super(...arguments);
    outlets.add(this.name);
    setProperties(this, {
      state: new State('idle'),
    });
    setProperties(this, {
      previousState: this.state,
    });
    this._listeners.add(this.router, {
      routeWillChange: transition => {
        const to = transition.to.name;
        const outlet =
          [...outlets].reverse().find(item => {
            return to.indexOf(item) !== -1;
          }) || 'application';
        if (this.name === outlet) {
          setProperties(this, {
            previousState: this.state,
            state: new State('loading'),
          });
        }
        if (this.root) {
          this.setAppState('loading');
        }
      },
      routeDidChange: transition => {
        setProperties(this, {
          // route: transition.to.name,
          previousState: this.state,
          state: new State('idle'),
        });
        if (this.root) {
          this.setAppRoute(this.router.currentRouteName);
          this.setAppState('idle');
        }
      },
    });
  },
});
