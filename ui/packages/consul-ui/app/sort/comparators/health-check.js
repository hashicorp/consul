/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default ({ properties }) =>
  (key = 'Status:asc') => {
    if (key.startsWith('Status:')) {
      return function (itemA, itemB) {
        const [, dir] = key.split(':');
        let a, b;
        if (dir === 'asc') {
          a = itemA;
          b = itemB;
        } else {
          b = itemA;
          a = itemB;
        }
        const statusA = a.Status;
        const statusB = b.Status;
        switch (statusA) {
          case 'passing':
            // a = passing
            // unless b is also passing then a is less important
            return statusB === 'passing' ? 0 : 1;
          case 'critical':
            // a = critical
            // unless b is also critical then a is more important
            return statusB === 'critical' ? 0 : -1;
          case 'warning':
            // a = warning
            switch (statusB) {
              // b is passing so a is more important
              case 'passing':
                return -1;
              // b is critical so a is less important
              case 'critical':
                return 1;
              // a and b are both warning, therefore equal
              default:
                return 0;
            }
        }
        return 0;
      };
    }
    return properties(['Name', 'Kind'])(key);
  };
