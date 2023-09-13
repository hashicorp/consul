/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import BaseAbility from './base';
import { inject as service } from '@ember/service';

import { isLegacy } from 'consul-ui/helpers/token/is-legacy';
import { isAnonymous } from 'consul-ui/helpers/token/is-anonymous';

export default class TokenAbility extends BaseAbility {
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
    return (
      this.env.var('CONSUL_ACLS_ENABLED') &&
      !isAnonymous([this.item]) &&
      this.item.AccessorID !== this.token.AccessorID &&
      super.canDelete
    );
  }

  get canDuplicate() {
    return this.env.var('CONSUL_ACLS_ENABLED') && !isLegacy([this.item]) && super.canWrite;
  }
}
