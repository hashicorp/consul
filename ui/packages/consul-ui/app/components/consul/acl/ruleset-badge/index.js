/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';

export default class ConsulAclRulesetBadgeComponent extends Component {
  get type() {
    return this.args.type || '';
  }

  get label() {
    return this.args.item?.Name || '';
  }

  get ariaLabel() {
    switch (this.type) {
      case 'policy-service-identity':
        return `Service Identity: ${this.label}`;
      case 'policy-node-identity':
        return `Node Identity: ${this.label}`;
      default:
        return this.label;
    }
  }

  get icon() {
    switch (this.type) {
      case 'policy-management':
        return 'star-fill';
      case 'read-only':
        return 'lock';
      case 'policy':
        return 'file-text';
      case 'role':
        return 'user';
      default:
        return null;
    }
  }

  get hasIcon() {
    return Boolean(this.icon);
  }
}
