import format from 'pretty-ms';
import { assert } from '@ember/debug';

import Service from '@ember/service';

export default class TemporalService extends Service {
  durationFrom(value, options = {}) {
    switch (true) {
      case typeof value === 'number':
        // if its zero, don't format just return zero as a string
        if (value === 0) {
          return '0';
        }
        return format(value / 1000000, { formatSubMilliseconds: true })
          .split(' ')
          .join('');
    }
    assert(`${value} is not a valid type`, false);
    return value;
  }
}
