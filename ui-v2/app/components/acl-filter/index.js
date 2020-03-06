import Component from '@ember/component';

export default Component.extend({
  tagName: 'form',
  classNames: ['filter-bar'],
  'data-test-acl-filter': true,
  onchange: function() {},
});
