/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Service from '@ember/service';
import atob from 'consul-ui/utils/atob';
export default class AtobService extends Service {
  execute() {
    return atob(...arguments);
  }
}
