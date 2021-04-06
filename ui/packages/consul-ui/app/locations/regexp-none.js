import Location from './regexp';

class FakeHistory {
  state = {};
  constructor(location, listener = () => {}) {
    this.listener = listener;
    this.location = location;
  }
  pushState(state, _, path) {
    this.state = state;
    this.location.pathname = path;

    this.listener({ state: this.state });
  }
  replaceState() {
    return this.pushState(...arguments);
  }
}
export default class extends Location {
  implementation = 'regexp-none';
  static create() {
    return new this(...arguments);
  }
  constructor() {
    super(...arguments);
    this.location = {
      pathname: '',
      search: '',
      hash: '',
    };
    this.history = new FakeHistory(this.location);
    this.doc = {
      defaultView: {
        addEventListener: (event, cb) => {
          this.history = new FakeHistory(this.location, cb);
        },
        removeEventListener: (event, cb) => {
          this.history = new FakeHistory();
        },
      },
    };
  }
}
