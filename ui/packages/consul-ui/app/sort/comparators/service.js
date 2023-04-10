/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default ({ properties }) =>
  (key = 'Status:asc') => {
    if (key.startsWith('Status:')) {
      return function (serviceA, serviceB) {
        const [, dir] = key.split(':');
        let a, b;
        if (dir === 'asc') {
          b = serviceA;
          a = serviceB;
        } else {
          a = serviceA;
          b = serviceB;
        }
        switch (true) {
          case a.MeshChecksCritical > b.MeshChecksCritical:
            return 1;
          case a.MeshChecksCritical < b.MeshChecksCritical:
            return -1;
          default:
            switch (true) {
              case a.MeshChecksWarning > b.MeshChecksWarning:
                return 1;
              case a.MeshChecksWarning < b.MeshChecksWarning:
                return -1;
              default:
                switch (true) {
                  case a.MeshChecksPassing < b.MeshChecksPassing:
                    return 1;
                  case a.MeshChecksPassing > b.MeshChecksPassing:
                    return -1;
                }
            }
            return 0;
        }
      };
    }
    return properties(['Name'])(key);
  };
