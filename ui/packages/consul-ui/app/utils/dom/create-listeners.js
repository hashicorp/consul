/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

class Listeners {
  constructor(listeners = []) {
    this.listeners = listeners;
  }
  add(target, event, handler) {
    let remove;
    if (typeof target === 'function') {
      remove = target;
    } else if (target instanceof Listeners) {
      remove = target.remove.bind(target);
    } else {
      let addEventListener = 'addEventListener';
      let removeEventListener = 'removeEventListener';
      if (typeof target[addEventListener] === 'undefined') {
        addEventListener = 'on';
        removeEventListener = 'off';
      }
      let obj = event;
      if (typeof obj === 'string') {
        obj = {
          [event]: handler,
        };
      }
      const removers = Object.keys(obj).map(function (key) {
        return (function (event, handler) {
          target[addEventListener](event, handler);
          return function () {
            target[removeEventListener](event, handler);
            return handler;
          };
        })(key, obj[key]);
      });
      // TODO: if event was a string only return the first
      // although the problem remains that it could sometimes return
      // a function, sometimes an array, so this needs some more thought
      remove = () => removers.map((item) => item());
    }
    this.listeners.push(remove);
    return () => {
      const pos = this.listeners.findIndex(function (item) {
        return item === remove;
      });
      return this.listeners.splice(pos, 1)[0]();
    };
  }
  remove() {
    const handlers = this.listeners.map((item) => item());
    this.listeners.splice(0, this.listeners.length);
    return handlers;
  }
}
export default function (listeners = []) {
  return new Listeners(listeners);
}
