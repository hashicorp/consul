/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Service from '@ember/service';

import Clipboard from 'clipboard';

export default class OsService extends Service {
  execute() {
    return new Clipboard(...arguments);
  }
}
