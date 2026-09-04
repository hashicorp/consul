/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { action } from '@ember/object';
import { cancel, later } from '@ember/runloop';
import chart from './chart.xstate';

// How long the "copied" feedback (green check icon) stays visible before
// reverting to the idle clipboard icon.
const RESET_DELAY = 2500;

export default class ConsulCopyButton extends Component {
  constructor() {
    super(...arguments);
    this.chart = chart;
  }

  // Show the success/error feedback, then revert to idle after a short delay.
  // The tooltip uses a hover trigger, so when RESET runs its text simply swaps
  // back to "Copy" while staying on screen (if still hovered) rather than
  // disappearing.
  @action
  flash(dispatch, reset) {
    dispatch();
    cancel(this._resetTimer);
    this._resetTimer = later(() => reset(), RESET_DELAY);
  }

  willDestroy() {
    super.willDestroy(...arguments);
    cancel(this._resetTimer);
  }
}
