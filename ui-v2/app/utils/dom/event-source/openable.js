/**
 * Wraps an EventSource so that you can `close` and `reopen`
 *
 * @param {Class} eventSource - EventSource class to extend from
 */
export default function(eventSource = EventSource) {
  return class extends eventSource {
    constructor(source, configuration) {
      super(...arguments);
      this.configuration = configuration;
    }
    open() {
      switch (this.readyState) {
        case 3: // CLOSING
          this.readyState = 1;
          break;
        case 2: // CLOSED
          eventSource.apply(this, [this.source, this.configuration]);
          break;
      }
      return this;
    }
  };
}
