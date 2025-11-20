/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';

export default class DcServicesShowController extends Controller {
  @tracked chain = undefined;
  @tracked proxies = [];

  @action setProxies(data) {
    
    // Ensure it's always an array
    this.proxies = Array.isArray(data) ? data : (data ? [data] : []);
  }

  @action setChain(data) {
    this.chain = data;
  }
}