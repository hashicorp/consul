import { schema } from 'consul-ui/models/peer';

export default ({ properties }) =>
  (key = 'State:asc') => {
    if (key.startsWith('State:')) {
      return function (itemA, itemB) {
        const [, dir] = key.split(':');
        let a, b;
        if (dir === 'asc') {
          b = itemA;
          a = itemB;
        } else {
          a = itemA;
          b = itemB;
        }
        switch (true) {
          case schema.State.allowedValues.indexOf(a.State) <
            schema.State.allowedValues.indexOf(b.State):
            return 1;
          case schema.State.allowedValues.indexOf(a.State) >
            schema.State.allowedValues.indexOf(b.State):
            return -1;
          case schema.State.allowedValues.indexOf(a.State) ===
            schema.State.allowedValues.indexOf(b.State):
            return 0;
        }
      };
    }
    return properties(['Name'])(key);
  };
