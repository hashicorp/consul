export default function(EventTarget, P = Promise) {
  const handler = function(e) {
    if (e.key === this.configuration.key) {
      P.resolve(this.getCurrentEvent()).then(event => {
        this.configuration.cursor++;
        this.dispatchEvent(event);
      });
    }
  };
  return class extends EventTarget {
    constructor(cb, configuration) {
      super(...arguments);
      this.source = cb;
      this.handler = handler.bind(this);
      this.configuration = configuration;
      this.configuration.cursor = 1;
      this.dispatcher = configuration.dispatcher;
      this.open();
    }
    dispatchEvent() {
      if (this.readyState === 1) {
        return super.dispatchEvent(...arguments);
      }
    }
    close() {
      this.dispatcher.removeEventListener('storage', this.handler);
      this.readyState = 2;
    }
    reopen() {
      this.dispatcher.addEventListener('storage', this.handler);
      this.readyState = 1;
    }
    getCurrentEvent() {
      return {
        type: 'message',
        data: this.source(this.configuration),
      };
    }
  };
}
