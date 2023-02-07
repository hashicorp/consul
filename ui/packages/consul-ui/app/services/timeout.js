import Service from '@ember/service';
import promisedTimeoutFactory from 'consul-ui/utils/promisedTimeout';
import { next } from '@ember/runloop';

const promisedTimeout = promisedTimeoutFactory(Promise);
export default class TimeoutService extends Service {
  // TODO: milliseconds should default to 0 or potentially just null
  // if it is 0/null use tick/next instead
  // if Octane eliminates the runloop things, look to use raf here instead
  execute(milliseconds, cb) {
    return promisedTimeout(milliseconds, cb);
  }

  tick() {
    return new Promise(function (resolve, reject) {
      next(resolve);
    });
  }
}
