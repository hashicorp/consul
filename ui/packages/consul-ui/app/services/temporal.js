import format from 'pretty-ms';
import { assert } from '@ember/debug';

import Service from '@ember/service';

export default class TemporalService extends Service {
  durationFrom(value, options = {}) {
    switch (true) {
      case typeof value === 'number':
        return format(value / 1000000, { formatSubMilliseconds: true })
          .split(' ')
          .join('');
    }
    assert(`${value} is not a valid type`, true);
    return value;
  }
}
