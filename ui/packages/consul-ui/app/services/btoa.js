/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Service from '@ember/service';
import btoa from 'consul-ui/utils/btoa';
export default class BtoaService extends Service {
  execute() {
    return btoa(...arguments);
  }
}
