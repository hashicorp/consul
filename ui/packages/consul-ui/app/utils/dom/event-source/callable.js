export const defaultRunner = function (target, configuration, isClosed) {
  if (isClosed(target)) {
    target.dispatchEvent({ type: 'close' });
    return;
  }
  // TODO Consider wrapping this is a promise for none thenable returns
  return target.source
    .bind(target)(configuration, target)
    .then(function (res) {
      return defaultRunner(target, configuration, isClosed);
    });
};
const errorEvent = function (e) {
  return new ErrorEvent('error', {
    error: e,
    message: e.message,
  });
};
const isClosed = function (target) {
  switch (target.readyState) {
    case 2: // CLOSED
    case 3: // CLOSING
      return true;
  }
  return false;
};
export default function (
  EventTarget,
  P = Promise,
  run = defaultRunner,
  createErrorEvent = errorEvent
) {
  const CallableEventSource = function (source, configuration = {}) {
    EventTarget.call(this);
    this.readyState = 2;
    this.source =
      typeof source !== 'function'
        ? function (configuration, target) {
            this.close();
            return P.resolve();
          }
        : source;
    this.readyState = 0; // CONNECTING
    P.resolve()
      .then(() => {
        // if we are already closed, don't do anything
        if (this.readyState > 1) {
          return;
        }
        this.readyState = 1; // open
        // the connection _was just_ opened
        this.dispatchEvent({ type: 'open' });
        return run(this, configuration, isClosed);
      })
      .catch((e) => {
        this.dispatchEvent(createErrorEvent(e));
        // close after the dispatch so we can tell if it was an error whilst closed or not
        // but make sure its before the promise tick
        this.readyState = 2; // CLOSE
        this.dispatchEvent({ type: 'close', error: e });
      })
      .then(() => {
        // This only gets called when the promise chain completely finishes
        // so only when its completely closed.
        this.readyState = 2; // CLOSE
      });
  };
  CallableEventSource.prototype = Object.assign(
    Object.create(EventTarget.prototype, {
      constructor: {
        value: CallableEventSource,
        configurable: true,
        writable: true,
      },
    }),
    {
      close: function () {
        // additional readyState 3 = CLOSING
        switch (this.readyState) {
          case 0: // CONNECTING
          case 2: // CLOSED
            this.readyState = 2;
            break;
          default:
            // OPEN
            this.readyState = 3; // CLOSING
        }
        // non-standard
        return this;
      },
    }
  );
  return CallableEventSource;
}
