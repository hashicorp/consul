export const defaultRunner = function(target, configuration, isClosed) {
  if (isClosed(target)) {
    target.dispatchEvent({ type: 'close' });
    return;
  }
  // TODO Consider wrapping this is a promise for none thenable returns
  return target.source
    .bind(target)(configuration)
    .then(function(res) {
      return defaultRunner(target, configuration, isClosed);
    });
};
const errorEvent = function(e) {
  return new ErrorEvent('error', {
    error: e,
    message: e.message,
  });
};
const isClosed = function(target) {
  switch (target.readyState) {
    case 2: // CLOSED
    case 3: // CLOSING
      return true;
  }
  return false;
};
export default function(
  EventTarget,
  P = Promise,
  run = defaultRunner,
  createErrorEvent = errorEvent
) {
  return class extends EventTarget {
    constructor(source, configuration = {}) {
      super();
      this.readyState = 2;
      this.source =
        typeof source !== 'function'
          ? function(configuration) {
              this.close();
              return P.resolve();
            }
          : source;
      this.readyState = 0; // connecting
      P.resolve()
        .then(() => {
          this.readyState = 1; // open
          // ...that the connection _was just_ opened
          this.dispatchEvent({ type: 'open' });
          return run(this, configuration, isClosed);
        })
        .catch(e => {
          this.dispatchEvent(createErrorEvent(e));
          // close after the dispatch so we can tell if it was an error whilst closed or not
          // but make sure its before the promise tick
          this.readyState = 2; // CLOSE
        })
        .then(() => {
          // This only gets called when the promise chain completely finishes
          // so only when its completely closed.
          this.readyState = 2; // CLOSE
        });
    }
    close() {
      // additional readyState 3 = CLOSING
      if (this.readyState !== 2) {
        this.readyState = 3;
      }
    }
  };
}
