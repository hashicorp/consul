import format from 'pretty-ms';
import parse from 'parse-duration';
import { assert } from '@ember/debug';
import dayjs from 'dayjs';
import relativeTime from 'dayjs/plugin/relativeTime';
import Service from '@ember/service';

dayjs.extend(relativeTime);

export default class TemporalService extends Service {
  format(value, options) {
    const djs = dayjs(value);
    if (dayjs().isBefore(djs)) {
      return dayjs().to(djs, true);
    } else {
      return dayjs().from(djs, true);
    }
  }

  within([value, d], options) {
    return dayjs(value).isBefore(dayjs().add(d, 'ms'));
  }

  parse(value, options) {
    return parse(value);
  }

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
      case typeof value === 'string':
        return value;
      default:
        assert(`${value} is not a valid type`, false);
        return value;
    }
  }
}
