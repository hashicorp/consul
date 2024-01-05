/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Service from '@ember/service';
import { once } from 'consul-ui/utils/dom/event-source';

export default class PromiseService extends Service {
  source(find, configuration) {
    return once(find, configuration);
  }
}
