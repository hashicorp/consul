import Component from '@ember/component';
export default Component.extend({
  tagName: 'fieldset',
  classNames: ['freetext-filter'],
  didInsertElement: function() {
    if (this.searchable) {
      this.onchange({ target: this });
    }
  },
  onchange: function(e) {
    let searchable = this.searchable;
    if (!Array.isArray(searchable)) {
      searchable = [searchable];
    }
    searchable.forEach(function(item) {
      item.search(e.target.value);
    });
  },
});
