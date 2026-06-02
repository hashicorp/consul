/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Helper from '@ember/component/helper';
import { css } from '@lit/reactive-element';

export default class ConsoleLogHelper extends Helper {
  compute([str], hash) {
    return css([str]);
  }
}
