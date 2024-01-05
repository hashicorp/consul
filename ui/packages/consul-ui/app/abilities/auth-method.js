/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import BaseAbility from './base';
import { inject as service } from '@ember/service';

export default class AuthMethodAbility extends BaseAbility {
  @service('env') env;

  resource = 'acl';
  segmented = false;

  get canRead() {
    return this.env.var('CONSUL_ACLS_ENABLED') && super.canRead;
  }

  get canCreate() {
    return this.env.var('CONSUL_ACLS_ENABLED') && super.canCreate;
  }

  get canDelete() {
    return this.env.var('CONSUL_ACLS_ENABLED') && super.canDelete;
  }

  get canUse() {
    return this.env.var('CONSUL_SSO_ENABLED');
  }
}
