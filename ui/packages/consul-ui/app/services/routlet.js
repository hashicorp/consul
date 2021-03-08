import Service from '@ember/service';
import { schedule } from '@ember/runloop';

class Outlets {
  constructor() {
    this.map = new Map();
    this.sorted = [];
  }
  sort() {
    this.sorted = [...this.map.keys()];
    this.sorted.sort((a, b) => {
      if (a === 'application') {
        return 1;
      }
      if (b === 'application') {
        return -1;
      }
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
    // TODO: find, splice to insert at the correct index instead of sorting
    // all the time
    this.sort();
  }
  get(name) {
    return this.map.get(name);
  }
  delete(name) {
    // TODO: find, splice to delete at the correct index instead of sorting
    // all the time
    this.map.delete(name);
    this.sort();
  }
  keys() {
    return this.sorted;
  }
}
const outlets = new Outlets();
export default class RoutletService extends Service {
  ready() {
    return this._transition;
  }

  transition() {
    let endTransition;
    this._transition = new Promise(resolve => {
      endTransition = resolve;
    });
    return endTransition;
  }

  findOutlet(name) {
    const keys = [...outlets.keys()];
    const key = keys.find(item => name.indexOf(item) !== -1);
    return key;
  }

  addOutlet(name, outlet) {
    outlets.set(name, outlet);
  }

  removeOutlet(name) {
    outlets.delete(name);
  }

  // modelFor gets the model for Outlet specified by `name`, not the Route
  modelFor(name) {
    const outlet = outlets.get(name);
    if (typeof outlet !== 'undefined') {
      return outlet.model || {};
    }
    return {};
  }

  addRoute(name, route) {
    const keys = [...outlets.keys()];
    const pos = keys.indexOf(name);
    const key = pos + 1;
    const outlet = outlets.get(keys[key]);
    if (typeof outlet !== 'undefined') {
      route.model = outlet.model;
      // TODO: Try to avoid the double computation bug
      schedule('afterRender', () => {
        outlet.routeName = route.args.name;
      });
    }
  }

  removeRoute(name, route) {}
}
