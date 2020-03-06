import Component from '@ember/component';

export default Component.extend({
  tagName: 'form',
  classNames: ['filter-bar'],
  'data-test-intention-filter': true,
  onchange: function() {},
});
