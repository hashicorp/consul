import rootKey from 'consul-ui/utils/rootKey';
// temporarily move this into a centralized place
// avoiding mixins and extending for the moment
// chances are this could also go
export default function(dc, parent, root) {
  return this.get('repo')
    .findAllBySlug(rootKey(parent, root), dc)
    .then(data => {
      this.transitionTo('dc.kv.show', parent);
    })
    .catch(e => {
      if (e.errors[0].status == 404) {
        this.transitionTo('dc.kv.show', root);
      }
    });
}
