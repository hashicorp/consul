/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Service from '@ember/service';
import atob from 'consul-ui/utils/atob';
export default class AtobService extends Service {
  execute() {
    return atob(...arguments);
  }
}
