import Service from '@ember/service';
import { hash } from 'rsvp';

// default 'workflow' service
// basically a wrapper around RSVP.hash
// more to come
export default Service.extend({
  // really should be protected
  // but make sure I can get to it via property injection
  resolve: hash,
  execute: function(hashCallback) {
    // basically hash({items: repo.findItems()})
    // and wait for the promises in the has to resolve
    return this.resolve(hashCallback());
  },
});
