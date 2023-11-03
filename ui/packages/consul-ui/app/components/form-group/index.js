/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Component from '@glimmer/component';

export default class FormGroup extends Component {
  get name() {
    return this.args.name;
  }
}
