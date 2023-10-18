/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Service from '@ember/service';
import { runInDebug } from '@ember/debug';

export default class LoggerService extends Service {
  execute(obj) {
    runInDebug(() => {
      obj = typeof obj.error !== 'undefined' ? obj.error : obj;
      if (obj instanceof Error) {
        console.error(obj); // eslint-disable-line no-console
      } else {
        console.log(obj); // eslint-disable-line no-console
      }
    });
  }
}
