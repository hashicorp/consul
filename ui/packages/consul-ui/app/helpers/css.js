/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Helper from '@ember/component/helper';
import { css } from '@lit/reactive-element';

export default class ConsoleLogHelper extends Helper {
  compute([str], hash) {
    return css([str]);
  }
}
