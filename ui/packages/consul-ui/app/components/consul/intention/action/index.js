/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';

export default class ConsulIntentionActionComponent extends Component {
  get normalizedAction() {
    return (this.args.action || '').toLowerCase();
  }

  get text() {
    return this.args.label || 'App aware';
  }

  get color() {
    switch (this.normalizedAction) {
      case 'allow':
        return 'success';
      case 'deny':
        return 'critical';
      default:
        return 'neutral';
    }
  }

  get icon() {
    switch (this.normalizedAction) {
      case 'allow':
        return 'check';
      case 'deny':
        return 'x';
      default:
        return 'info';
    }
  }
}
