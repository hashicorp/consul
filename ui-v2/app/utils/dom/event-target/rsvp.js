// Simple RSVP.EventTarget wrapper to make it more like a standard EventTarget
import RSVP from 'rsvp';
// See https://github.com/mysticatea/event-target-shim/blob/v4.0.2/src/event.mjs
// The MIT License (MIT) - Copyright (c) 2015 Toru Nagashima
import { setCurrentTarget, wrapEvent } from './event-target-shim/event';

const EventTarget = function() {};
function callbacksFor(object) {
  let callbacks = object._promiseCallbacks;

  if (!callbacks) {
    callbacks = object._promiseCallbacks = {};
  }

  return callbacks;
}
EventTarget.prototype = Object.assign(
  Object.create(Object.prototype, {
    constructor: {
      value: EventTarget,
      configurable: true,
      writable: true,
    },
  }),
  {
    dispatchEvent: function(obj) {
      // borrow just what I need from event-target-shim
      // to make true events even ErrorEvents with targets
      const wrappedEvent = wrapEvent(this, obj);
      setCurrentTarget(wrappedEvent, null);
      // RSVP trigger doesn't bind to `this`
      // the rest is pretty much the contents of `trigger`
      // but with a `.bind(this)` to make it compatible
      // with standard EventTarget
      // we use  `let` and `callbacksFor` above, just to keep things the same as rsvp.js
      const eventName = obj.type;
      const options = wrappedEvent;
      let allCallbacks = callbacksFor(this);

      let callbacks = allCallbacks[eventName];
      if (callbacks) {
        // Don't cache the callbacks.length since it may grow
        let callback;
        for (let i = 0; i < callbacks.length; i++) {
          callback = callbacks[i];
          callback.bind(this)(options);
        }
      }
    },
    addEventListener: function(event, cb) {
      this.on(event, cb);
    },
    removeEventListener: function(event, cb) {
      try {
        this.off(event, cb);
      } catch (e) {
        // passthrough
      }
    },
  }
);
RSVP.EventTarget.mixin(EventTarget.prototype);
export default EventTarget;
