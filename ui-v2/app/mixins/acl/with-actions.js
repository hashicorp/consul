import Mixin from '@ember/object/mixin';
import { get } from '@ember/object';
import { inject as service } from '@ember/service';
import WithFeedback from 'consul-ui/mixins/with-feedback';

export default Mixin.create(WithFeedback, {
  settings: service('settings'),
  actions: {
    create: function(item) {
      get(this, 'feedback').execute(
        () => {
          return get(this, 'repo')
            .persist(item)
            .then(item => {
              return this.transitionTo('dc.acls');
            });
        },
        `Your ACL token has been added.`,
        `There was an error adding your ACL token.`
      );
    },
    update: function(item) {
      get(this, 'feedback').execute(
        () => {
          return get(this, 'repo')
            .persist(item)
            .then(() => {
              return this.transitionTo('dc.acls');
            });
        },
        `Your ACL token was saved.`,
        `There was an error saving your ACL token.`
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
                  case 'dc.acls.index':
                    return this.refresh();
                  default:
                    return this.transitionTo('dc.acls');
                }
              })
          );
        },
        `Your ACL token was deleted.`,
        `There was an error deleting your ACL token.`
      );
    },
    cancel: function(item) {
      this.transitionTo('dc.acls');
    },
    use: function(item) {
      get(this, 'feedback').execute(
        () => {
          return get(this, 'settings')
            .persist({ token: get(item, 'ID') })
            .then(() => {
              this.transitionTo('dc.services');
            });
        },
        `Now using new ACL token`,
        `There was an error using that ACL token`
      );
    },
    clone: function(item) {
      get(this, 'feedback').execute(
        () => {
          return get(this, 'repo')
            .clone(item)
            .then(item => {
              switch (this.routeName) {
                case 'dc.acls.index':
                  return this.refresh();
                default:
                  return this.transitionTo('dc.acls');
              }
            });
        },
        `Your ACL token was cloned.`,
        `There was an error cloning your ACL token.`
      );
    },
  },
});
