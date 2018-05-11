import Service from '@ember/service';
import promisedTimeoutFactory from 'consul-ui/utils/promisedTimeout';
import { Promise } from 'rsvp';
const promisedTimeout = promisedTimeoutFactory(Promise);
export default Service.extend({
  execute: function(milliseconds, cb) {
    return promisedTimeout(milliseconds, cb);
  },
});
