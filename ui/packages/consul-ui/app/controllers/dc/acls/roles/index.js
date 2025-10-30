/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */
import Controller from '@ember/controller';
import { action } from '@ember/object';

export default class DcAclsRolesIndexController extends Controller {
  @action
  delete(item) {
    this.target.send('delete', item);
  }
}
