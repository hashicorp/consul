/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import BaseAbility, { ACCESS_READ, ACCESS_WRITE } from './base';

export default class ServiceInstanceAbility extends BaseAbility {
  resource = 'service';
  generateForSegment(segment) {
    // When we ask for service-instances its almost like a request for a single service
    // When we do that we also want to know if we can read/write intentions for services
    // so here we add intentions read/write for the specific segment/service prefix
    return super
      .generateForSegment(...arguments)
      .concat([
        this.permissions.generate('intention', ACCESS_READ, segment),
        this.permissions.generate('intention', ACCESS_WRITE, segment),
      ]);
  }
}
