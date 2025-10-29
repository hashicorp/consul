/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */
import Route from 'consul-ui/routing/route';

export default class DcAclsIndexRoute extends Route {
  beforeModel() {
    this.replaceWith('dc.acls.tokens');
  }
}