export default function (EventTarget, P = Promise) {
  const handler = function (e) {
    // e is undefined on the opening call
    if (typeof e === 'undefined' || e.key === this.configuration.key) {
      if (this.readyState === 1) {
        const res = this.source(this.configuration);
        P.resolve(res).then((data) => {
          this.configuration.cursor++;
          this._currentEvent = { type: 'message', data: data };
          this.dispatchEvent({ type: 'message', data: data });
        });
      }
    }
  };
  return class extends EventTarget {
    constructor(cb, configuration) {
      super(...arguments);
      this.readyState = 2;
      this.target = configuration.target || window;
      this.name = 'storage';
      this.source = cb;
      this.handler = handler.bind(this);
      this.configuration = configuration;
      this.configuration.cursor = 1;
      this.open();
    }
    dispatchEvent() {
      if (this.readyState === 1) {
        return super.dispatchEvent(...arguments);
      }
    }
    close() {
      this.target.removeEventListener(this.name, this.handler);
      this.readyState = 2;
    }
    getCurrentEvent() {
      return this._currentEvent;
    }
    open() {
      const state = this.readyState;
      this.readyState = 1;
      if (state !== 1) {
        this.target.addEventListener(this.name, this.handler);
        this.handler();
      }
    }
  };
}
