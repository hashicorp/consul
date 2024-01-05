/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';

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
  @tracked state;
  @tracked previousState;
  @tracked endTransition;

  @tracked route;

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
      this.previousState = this.state;
      this.state = new State('loading');
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
    if (this.state.matches('loading')) {
      this.previousState = this.state;
      this.state = new State('idle');
    }
    if (this.args.name === 'application') {
      this.setAppRoute(this.router.currentRouteName);
    }
  }

  @action
  connect() {
    this.routlet.addOutlet(this.args.name, this);
    this.previousState = this.state = new State('idle');
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
