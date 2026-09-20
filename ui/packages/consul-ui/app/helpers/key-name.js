/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { helper } from '@ember/component/helper';
import keyName from 'consul-ui/utils/keyName';

export default helper(function ([path = ''], hash) {
  return keyName(path);
});
