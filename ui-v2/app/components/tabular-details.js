import Component from '@ember/component';
import SlotsMixin from 'ember-block-slots';
import closest from 'consul-ui/utils/dom/closest';
import clickFirstAnchorFactory from 'consul-ui/utils/dom/click-first-anchor';
const clickFirstAnchor = clickFirstAnchorFactory(closest);

export default Component.extend(SlotsMixin, {
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
