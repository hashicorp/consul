/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

// a simple state machine that the History API happens to more or less implement
// it should really be an EventTarget but what we need here is simple enough
export class FSM {
  // extends EventTarget/EventSource
  state = {};
  constructor(location, listener = () => {}) {
    this.listener = listener;
    this.location = location;
  }
  /**
   * @param state The infinite/extended state or context
   * @param _ `_` was meant to be title but was never used, don't use this
   *          argument for anything unless browsers change, see:
   *          https://github.com/whatwg/html/issues/2174
   * @param path The state/event
   */
  pushState(state, _, path) {
    this.state = state;
    this.location.pathname = path;
    this.listener({ state: this.state });
  }
  replaceState() {
    return this.pushState(...arguments);
  }
}

export class Location {
  pathname = '';
  search = '';
  hash = '';
}

export default class FSMLocation {
  implementation = 'fsm';
  static create() {
    return new this(...arguments);
  }
  constructor(owner) {
    this.container = Object.entries(owner)[0][1];
  }
  visit() {
    return this.transitionTo(...arguments);
  }
  hrefTo() {}
  transitionTo() {}
}
