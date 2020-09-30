import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';

class State {
  constructor(name) {
    this.name = name;
  }
  matches(match) {
    return this.name === match;
  }
}

class Outlets {
  constructor() {
    this.map = new Map();
  }
  sort() {
    this.sorted = [...this.map.keys()];
    this.sorted.sort((a, b) => {
      const al = a.split('.').length;
      const bl = b.split('.').length;
      switch (true) {
        case al > bl:
          return -1;
        case al < bl:
          return 1;
        default:
          return 0;
      }
    });
  }
  set(name, value) {
    this.map.set(name, value);
    this.sort();
  }
  get(name) {
    return this.map.get(name);
  }
  delete(name) {
    this.map.delete(name);
    this.sort();
  }
  keys() {
    return this.sorted;
  }
}
const outlets = new Outlets();

export default class Outlet extends Component {
  @service('router') router;
  @service('dom') dom;

  @tracked route;
  @tracked state;
  @tracked previousState;

  constructor() {
    super(...arguments);
    if (this.args.name === 'application') {
      this.setAppState('loading');
      this.setAppRoute(this.router.currentRouteName);
    }
  }

  setAppRoute(name) {
    if (name.startsWith('nspace.')) {
      name = name.substr(0, 'nspace.'.length);
    }
    if (name !== 'loading') {
      const doc = this.dom.root();
      if (doc.classList.contains('ember-loading')) {
        doc.classList.remove('ember-loading');
      }
      doc.dataset.route = name;
      this.setAppState('idle');
    }
  }

  setAppState(state) {
    this.dom.root().dataset.state = state;
  }

  setOutletRoutes(route) {
    const keys = [...outlets.keys()];
    const pos = keys.indexOf(this.name);
    const key = pos + 1;
    const parent = outlets.get(keys[key]);
    parent.route = this.args.name;

    this.route = route;
  }

  @action
  startLoad(transition) {
    const keys = [...outlets.keys()];

    const outlet =
      keys.find(item => {
        return transition.to.name.indexOf(item) !== -1;
      }) || 'application';

    if (this.args.name === outlet) {
      this.previousState = this.state;
      this.state = new State('loading');
    }
    if (this.args.name === 'application') {
      this.setAppState('loading');
    }
  }

  @action
  endLoad(transition) {
    if (this.state.matches('loading')) {
      this.setOutletRoutes(transition.to.name);

      this.previousState = this.state;
      this.state = new State('idle');
    }
    if (this.args.name === 'application') {
      this.setAppRoute(this.router.currentRouteName);
    }
  }

  @action
  connect() {
    outlets.set(this.args.name, this);
    this.previousState = this.state = new State('idle');
    this.router.on('routeWillChange', this.startLoad);
    this.router.on('routeDidChange', this.endLoad);
  }

  @action
  disconnect() {
    outlets.delete(this.args.name);
    this.router.off('routeWillChange', this.startLoad);
    this.router.off('routeDidChange', this.endLoad);
  }
}
