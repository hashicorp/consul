/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default ({ properties }) =>
  (key = 'Name:asc') => {
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
          case a.ChecksCritical > b.ChecksCritical:
            return 1;
          case a.ChecksCritical < b.ChecksCritical:
            return -1;
          default:
            switch (true) {
              case a.ChecksWarning > b.ChecksWarning:
                return 1;
              case a.ChecksWarning < b.ChecksWarning:
                return -1;
              default:
                switch (true) {
                  case a.ChecksPassing < b.ChecksPassing:
                    return 1;
                  case a.ChecksPassing > b.ChecksPassing:
                    return -1;
                }
            }
            return 0;
        }
      };
    } else if (key.startsWith('Version:')) {
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

        // Split the versions into arrays of numbers
        const versionA = a.Version.split('.').map((part) => {
          const number = Number(part);
          return isNaN(number) ? 0 : number;
        });
        const versionB = b.Version.split('.').map((part) => {
          const number = Number(part);
          return isNaN(number) ? 0 : number;
        });

        const minLength = Math.min(versionA.length, versionB.length);

        for (let i = 0; i < minLength; i++) {
          const diff = versionA[i] - versionB[i];
          switch (true) {
            case diff > 0:
              return 1;
            case diff < 0:
              return -1;
          }
        }

        return versionA.length - versionB.length;
      };
    }

    return properties(['Node'])(key);
  };
