import Component from '@ember/component';
import closest from 'consul-ui/utils/dom/closest';
import clickFirstAnchorFactory from 'consul-ui/utils/dom/click-first-anchor';
const clickFirstAnchor = clickFirstAnchorFactory(closest);

export default Component.extend({
  onchange: function() {},
  actions: {
    click: function(e) {
      clickFirstAnchor(e);
    },
    change: function(item, e) {
      this.onchange(e, item);
    },
  },
});
