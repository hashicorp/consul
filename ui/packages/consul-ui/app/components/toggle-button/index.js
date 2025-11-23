/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';
import { guidFor } from '@ember/object/internals';
import { inject as service } from '@ember/service';

export default class ToggleButtonComponent extends Component {
  @service dom;

  @tracked input;
  @tracked label;

  // Generate unique ID for this component instance
  guid = guidFor(this);

  get checked() {
    return this.args.checked || false;
  }

  get onchange() {
    return this.args.onchange || (() => {});
  }

  get onblur() {
    // TODO: reserved for the moment but we don't need it yet
    return this.args.onblur || (() => {});
  }

  @action
  click(e) {
    // only preventDefault if the target isn't an external link
    // TODO: this should be changed for an explicit close
    if ((e.target.rel || '').indexOf('noopener') === -1) {
      e.preventDefault();
    }
    this.input.checked = !this.input.checked;
    // manually dispatched mouse events have a detail = 0
    // real mouse events have the number of click counts
    if (e.detail !== 0) {
      e.target.blur();
    }
    this.change(e);
  }

  @action
  change(e) {
    // TODO: This should be an event
    this.onchange({ target: this.input });
  }

  @action
  setupInput(element) {
    this.input = element;
  }

  @action
  setupLabel(element) {
    this.label = element;
  }

  @action
  handleClickOutside() {
    if (this.input?.checked) {
      this.input.checked = false;
      // TODO: This should be an event
      this.onchange({ target: this.input });
    }
  }
}
