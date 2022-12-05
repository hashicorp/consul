/**
 * Wraps an EventSource so that you can `close` and `reopen`
 *
 * @param {Class} eventSource - EventSource class to extend from
 */
export default function (eventSource = EventSource) {
  const OpenableEventSource = function (source, configuration = {}) {
    eventSource.apply(this, arguments);
    this.configuration = configuration;
  };
  OpenableEventSource.prototype = Object.assign(
    Object.create(eventSource.prototype, {
      constructor: {
        value: OpenableEventSource,
        configurable: true,
        writable: true,
      },
    }),
    {
      open: function () {
        switch (this.readyState) {
          case 3: // CLOSING
            this.readyState = 1;
            break;
          case 2: // CLOSED
            eventSource.apply(this, [this.source, this.configuration]);
            break;
        }
        return this;
      },
    }
  );
  return OpenableEventSource;
}
