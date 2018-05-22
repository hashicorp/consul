import Mixin from '@ember/object/mixin';
import { get } from '@ember/object';
import WithFeedback from 'consul-ui/mixins/with-feedback';

export default Mixin.create(WithFeedback, {
  actions: {
    create: function(item) {
      get(this, 'feedback').execute(
        () => {
          return get(this, 'repo')
            .persist(item)
            .then(item => {
              return this.transitionTo('dc.intentions');
            });
        },
        `Your intention has been added.`,
        `There was an error adding your intention.`
      );
    },
    update: function(item) {
      get(this, 'feedback').execute(
        () => {
          return get(this, 'repo')
            .persist(item)
            .then(() => {
              return this.transitionTo('dc.intentions');
            });
        },
        `Your intention was saved.`,
        `There was an error saving your intention.`
      );
    },
    delete: function(item) {
      get(this, 'feedback').execute(
        () => {
          return (
            get(this, 'repo')
              // ember-changeset doesn't support `get`
              // and `data` returns an object not a model
              .remove(item)
              .then(() => {
                switch (this.routeName) {
                  case 'dc.intentions.index':
                    return this.refresh();
                  default:
                    return this.transitionTo('dc.intentions');
                }
              })
          );
        },
        `Your intention was deleted.`,
        `There was an error deleting your intention.`
      );
    },
    cancel: function(item) {
      this.transitionTo('dc.intentions');
    },
  },
});
