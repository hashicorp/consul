import Mixin from '@ember/object/mixin';

export default Mixin.create({
  rootKey: '-',
  actions: {
    // Used to link to keys that are not objects,
    // like parents and grandParents
    // TODO: This is a view thing, should possibly be a helper
    linkToKey: function(key) {
      if (key.slice(-1) === '/' || key === this.rootKey) {
        this.transitionTo('dc.kv.show', key);
      } else {
        this.transitionTo('dc.kv.edit', key);
      }
    },
  },
});
