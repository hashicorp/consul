/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';
import { registerDestructor } from '@ember/destroyable';

export default class ToggleButtonComponent extends Component {
  @service('dom') dom;

  guid = this.dom.guid(this);
  _listeners = this.dom.listeners();

  constructor(owner, args) {
    super(owner, args);
    registerDestructor(this, () => {
      this._listeners.remove();
    });
  }

  @action
  captureInput(element) {
    this.input = element;
  }

  @action
  captureLabel(element) {
    this.label = element;
    // Set up click outside listener if initially checked
    if (this.args.checked) {
      this.addClickOutsideListener();
    }
  }

  addClickOutsideListener() {
    // default onblur event
    this._listeners.remove();
    this._listeners.add(this.dom.document(), 'click', (e) => {
      if (this.dom.isOutside(this.label, e.target)) {
        if (this.dom.isOutside(this.label.nextElementSibling, e.target)) {
          if (this.input.checked) {
            this.input.checked = false;
            const onchange = this.args.onchange || (() => {});
            onchange({ target: this.input });
          }
          this._listeners.remove();
        }
      }
    });
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
    if (this.input.checked) {
      this.addClickOutsideListener();
    }
    const onchange = this.args.onchange || (() => {});
    onchange({ target: this.input });
  }
}
