/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Helper from './can';
import { is } from './is';

export default class extends Helper {
  compute([abilityString, model], properties) {
    switch (true) {
      case abilityString.startsWith('can '):
        return super.compute([abilityString.substr(4), model], properties);
      case abilityString.startsWith('is '):
        return is(this, [abilityString.substr(3), model], properties);
    }
    throw new Error(`${abilityString} is not supported by the 'test' helper.`);
  }
}
