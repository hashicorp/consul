/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { helper } from '@ember/component/helper';
import { MANAGEMENT_ID } from 'consul-ui/models/policy';

export default helper(function policyGroup([items] /*, hash*/) {
  return items.reduce(
    function (prev, item) {
      let group;
      switch (true) {
        case item?.ID === MANAGEMENT_ID:
          group = 'management';
          break;
        case item?.template !== '':
          group = 'identities';
          break;
        default:
          group = 'policies';
          break;
      }
      prev[group].push(item);
      return prev;
    },
    {
      management: [],
      identities: [],
      policies: [],
    }
  );
});
