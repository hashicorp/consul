import Component from '@ember/component';
import { get } from '@ember/object';
export default Component.extend({
  tagName: 'fieldset',
  classNames: ['freetext-filter'],
  onchange: function(e) {
    let searchable = get(this, 'searchable');
    if (!Array.isArray(searchable)) {
      searchable = [searchable];
    }
    searchable.forEach(function(item) {
      item.search(e.target.value);
    });
  },
});
