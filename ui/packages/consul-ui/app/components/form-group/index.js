/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';

export default class FormGroup extends Component {
  get name() {
    return this.args.name;
  }
}
