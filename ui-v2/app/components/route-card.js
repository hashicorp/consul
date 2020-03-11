import Component from '@ember/component';
import { get, computed } from '@ember/object';

export default Component.extend({
  tagName: '',
  path: computed('item', function() {
    return Object.entries(get(this, 'item.Definition.Match.HTTP') || {}).reduce(
      function(prev, [key, value]) {
        if (key.toLowerCase().startsWith('path')) {
          return {
            type: key.replace('Path', ''),
            value: value,
          };
        }
        return prev;
      },
      {
        type: 'Prefix',
        value: '/',
      }
    );
  }),
});
