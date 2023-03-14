/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import BaseAbility from './base';
import { inject as service } from '@ember/service';
import { typeOf } from 'consul-ui/helpers/policy/typeof';

export default class PolicyAbility extends BaseAbility {
  @service('env') env;

  resource = 'acl';
  segmented = false;

  get canRead() {
    return this.env.var('CONSUL_ACLS_ENABLED') && super.canRead;
  }

  get canWrite() {
    return (
      this.env.var('CONSUL_ACLS_ENABLED') &&
      (typeof this.item === 'undefined' || typeOf([this.item]) !== 'policy-management') &&
      super.canWrite
    );
  }

  get canCreate() {
    return this.env.var('CONSUL_ACLS_ENABLED') && super.canCreate;
  }

  get canDelete() {
    return (
      this.env.var('CONSUL_ACLS_ENABLED') &&
      (typeof this.item === 'undefined' || typeOf([this.item]) !== 'policy-management') &&
      super.canDelete
    );
  }
}
