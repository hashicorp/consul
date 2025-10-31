/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';
import { schedule } from '@ember/runloop';

class State {
  constructor(name) {
    this.name = name;
  }
  matches(match) {
    return this.name === match;
  }
}

export default class Outlet extends Component {
  @service('routlet') routlet;
  @service('router') router;

  @tracked element;
  @tracked routeName;
  @tracked _state = new State('idle');
  @tracked _previousState = new State('idle');
  @tracked endTransition;
  // Non-tracked staging variable (won't trigger tracking violations)
  _stagingState = null;

  @tracked route;
  get state() {
    return this._stagingState || this._state;
  }

  get previousState() {
    return this._previousState;
  }

  get model() {
    return this.args.model || {};
  }

  get name() {
    return this.args.name;
  }

  setAppRoute(name) {
    if (name !== 'loading' || name === 'oidc-provider-debug') {
      const doc = this.element.ownerDocument.documentElement;
      if (doc.classList.contains('ember-loading')) {
        doc.classList.remove('ember-loading');
      }
      doc.dataset.route = name;
      this.setAppState('idle');
    }
  }

  setAppState(state) {
    const doc = this.element.ownerDocument.documentElement;
    doc.dataset.state = state;
  }

  @action
  attributeChanged(prop, value) {
    switch (prop) {
      case 'element':
        this.element = value;
        if (this.args.name === 'application') {
          this.setAppState('loading');
          this.setAppRoute(this.router.currentRouteName);
        }
        break;
      case 'model':
        if (typeof this.route !== 'undefined') {
          this.route._model = value;
        }
        break;
    }
  }

  @action transitionEnd($el) {
    if (typeof this.endTransition === 'function') {
      this.endTransition();
    }
  }

  @action
  startLoad(transition) {
    const outlet = this.routlet.findOutlet(transition.to.name) || 'application';
    if (this.args.name === outlet) {
      this._stagingState = new State('loading');
      // Commit after render
      schedule('afterRender', () => {
        this._previousState = this._state;

        this._state = this._stagingState || new State('loading');
        this._stagingState = null;
      });
      this.endTransition = this.routlet.transition();
      let duration;
      if (this.element) {
        // if we have no transition-duration set immediately end the transition
        duration = window.getComputedStyle(this.element).getPropertyValue('transition-duration');
      } else {
        duration = 0;
      }

      if (parseFloat(duration) === 0) {
        this.endTransition();
      }
    }
    if (this.args.name === 'application') {
      this.setAppState('loading');
    }
  }

  @action
  endLoad(transition) {
    const currentState = this._stagingState || this._state;
    if (currentState.matches('loading')) {
      this._stagingState = new State('idle');
      schedule('afterRender', () => {
        this._previousState = this._state;
        this._state = this._stagingState || new State('idle');
        this._stagingState = null;
      });
    }
    if (this.args.name === 'application') {
      this.setAppRoute(this.router.currentRouteName);
    }
  }

  @action
  connect() {
    this.routlet.addOutlet(this.args.name, this);
    this.router.on('routeWillChange', this.startLoad);
    this.router.on('routeDidChange', this.endLoad);
  }

  @action
  disconnect() {
    this.routlet.removeOutlet(this.args.name);
    this.router.off('routeWillChange', this.startLoad);
    this.router.off('routeDidChange', this.endLoad);
  }
}
