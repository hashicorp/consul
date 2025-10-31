/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */
import Controller from '@ember/controller';
import { action } from '@ember/object';

export default class DcAclsPoliciesIndexController extends Controller {
  @action
  delete(item) {
    // delegate to the route (WithBlockingActions mixin) which has the delete action
    this.target.send('delete', item);
  }
}
