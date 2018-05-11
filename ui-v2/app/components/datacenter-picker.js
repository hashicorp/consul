import Component from '@ember/component';
import WithClickOutside from 'consul-ui/mixins/click-outside';

export default Component.extend(WithClickOutside, {
  tagName: 'ul',
  onchange: function() {},
});
