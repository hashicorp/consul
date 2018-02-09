import Route from '@ember/routing/route';

export function initialize(/* application */) {
  Route.reopen({
    rootKey: "-",
    condensedView: false,
    // Don't record characters in browser history
    // for the "search" query item (filter)
    // queryParams: {
    //   filter: {
    //     replace: true
    //   }
    // },
    getParentAndGrandparent: function(key) {
      var parentKey = this.rootKey,
        grandParentKey = this.rootKey,
        parts = key.split('/');

      if (parts.length > 0) {
        parts.pop();
        parentKey = parts.join("/") + "/";
      }
      if (parts.length > 1) {
        parts.pop();
        grandParentKey = parts.join("/") + "/";
      }
      return {
        parent: parentKey,
        grandParent: grandParentKey,
        isRoot: parentKey === '/'
      };
    },
    removeDuplicateKeys: function(keys, matcher) {
      // Loop over the keys
      keys.forEach(function(item, index) {
        if (item.get('Key') == matcher) {
          // If we are in a nested folder and the folder
          // name matches our position, remove it
          keys.splice(index, 1);
        }
      });
      return keys;
    },
    actions: {
      // Used to link to keys that are not objects,
      // like parents and grandParents
      linkToKey: function(key) {
        if (key == "/") {
          this.transitionTo('kv.show', "-");
        } else if (key.slice(-1) === '/' || key === this.rootKey) {
          this.transitionTo('kv.show', key);
        } else {
          this.transitionTo('kv.edit', key);
        }
      }
    }
  });
}

export default {
  initialize
};
