import Mixin from '@ember/object/mixin';
import { inject as service } from '@ember/service';
import { get, set } from '@ember/object';
/** With Blocking Actions
 * This mixin contains common write actions (Create Update Delete) for routes.
 * It could also be an Route to extend but decoration seems to be more sense right now.
 *
 * Each 'blocking action' (blocking in terms of showing some sort of blocking loader) is
 * wrapped in the functionality to signal that the page should be blocked
 * (currently via the 'feedback' service) as well as some sane default hooks for where the page
 * should go when the action has finished.
 *
 * Hooks can and are being overwritten for custom redirects/error handling on a route by route basis.
 *
 * Notifications are part of the injectable 'feedback' service, meaning different ones
 * could be easily swapped in an out as need be in the future.
 *
 */
export default Mixin.create({
  _feedback: service('feedback'),
  init: function() {
    this._super(...arguments);
    const feedback = get(this, '_feedback');
    const route = this;
    set(this, 'feedback', {
      execute: function(cb, type, error) {
        return feedback.execute(cb, type, error, route.controller);
      },
    });
  },
  afterCreate: function(item) {
    // do the same as update
    return this.afterUpdate(...arguments);
  },
  afterUpdate: function(item) {
    // e.g. dc.intentions.index
    const parts = this.routeName.split('.');
    // e.g. index or edit
    parts.pop();
    // e.g. dc.intentions, essentially go to the listings page
    return this.transitionTo(parts.join('.'));
  },
  afterDelete: function(item) {
    // e.g. dc.intentions.index
    const parts = this.routeName.split('.');
    // e.g. index or edit
    const page = parts.pop();
    switch (page) {
      case 'index':
        // essentially if I'm on the index page, stay there
        return this.refresh();
      default:
        // e.g. dc.intentions essentially do to the listings page
        return this.transitionTo(parts.join('.'));
    }
  },
  errorCreate: function(type, e) {
    return type;
  },
  errorUpdate: function(type, e) {
    return type;
  },
  errorDelete: function(type, e) {
    return type;
  },
  actions: {
    cancel: function() {
      // do the same as an update, or override
      return this.afterUpdate(...arguments);
    },
    create: function(item) {
      return get(this, 'feedback').execute(
        () => {
          return get(this, 'repo')
            .persist(item)
            .then(item => {
              return this.afterCreate(...arguments);
            });
        },
        'create',
        (type, e) => {
          return this.errorCreate(type, e);
        }
      );
    },
    update: function(item) {
      return get(this, 'feedback').execute(
        () => {
          return get(this, 'repo')
            .persist(item)
            .then(() => {
              return this.afterUpdate(...arguments);
            });
        },
        'update',
        (type, e) => {
          return this.errorUpdate(type, e);
        }
      );
    },
    delete: function(item) {
      return get(this, 'feedback').execute(
        () => {
          return get(this, 'repo')
            .remove(item)
            .then(() => {
              return this.afterDelete(...arguments);
            });
        },
        'delete',
        (type, e) => {
          return this.errorDelete(type, e);
        }
      );
    },
  },
});
