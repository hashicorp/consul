/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';
import { registerDestructor } from '@ember/destroyable';

function cleanup(instance) {
  if (instance.args.select) {
    instance.args.select.removeOption(instance);
  }
}

export default class Option extends Component {
  @tracked selected;

  constructor(owner, args) {
    super(owner, args);
    this.selected = args.selected;
    registerDestructor(this, cleanup);
  }

  @action
  connect(e) {
    this.updateSelected();
    this.args.select.addOption(this);
  }

  @action
  updateSelected() {
    this.selected = this.args.selected;
  }
}
