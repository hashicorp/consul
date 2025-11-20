/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */
import Helper from '@ember/component/helper';
import { inject as service } from '@ember/service';

export default class DomGuidHelper extends Helper {
  @service('dom') dom;
  compute() {
    return this.dom.guid({});
  }
}