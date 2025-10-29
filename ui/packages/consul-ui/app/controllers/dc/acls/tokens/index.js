/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */
import Controller from '@ember/controller';
import { action } from '@ember/object';

export default class DcAclsTokensIndexController extends Controller {
  @action onUse(item) {
    this.target.send('use', item);
  }
  @action onDelete(item) {
    this.target.send('delete', item);
  }
  @action onLogout(item) {
    this.target.send('logout', item);
  }
  @action onClone(item) {
    this.target.send('clone', item);
  }
}
