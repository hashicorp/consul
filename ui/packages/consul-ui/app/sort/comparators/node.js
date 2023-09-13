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
    }
    return properties(['Node'])(key);
  };
