import rootKey from 'consul-ui/utils/rootKey';
import error from 'consul-ui/utils/error';
import { Promise } from 'rsvp';
import { get } from '@ember/object';
// temporarily move this into a centralized place
// avoiding mixins and extending for the moment
// chances are this could also go
export default function(dc, parent, root) {
  if (parent === '/') {
    return Promise.resolve(this.transitionTo('dc.kv.show', root));
  }
  return get(this, 'repo')
    .findAllBySlug(rootKey(parent, root), dc)
    .then(data => {
      return this.transitionTo('dc.kv.show', parent);
    })
    .catch(e => {
      if (e.errors && e.errors[0].status == 404) {
        return this.transitionTo('dc.kv.show', root);
      } else {
        error(e);
      }
    });
}
