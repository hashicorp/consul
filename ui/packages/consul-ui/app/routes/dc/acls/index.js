/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */
import Route from 'consul-ui/routing/route';
import { inject as service } from '@ember/service';

export default class DcAclsIndexRoute extends Route {
  @service router;

  beforeModel() {
    this.router.replaceWith('dc.acls.tokens');
  }
}
