import { computed, get } from '@ember/object';
import Component from 'ember-collection/components/ember-collection';
import style from 'ember-computed-style';
import WithResizing from 'consul-ui/mixins/with-resizing';
import qsaFactory from 'consul-ui/utils/qsa-factory';
const $$ = qsaFactory();

export default Component.extend(WithResizing, {
  tagName: 'div',
  attributeBindings: ['style'],
  height: 500,
  style: style('getStyle'),
  classNames: ['list-collection'],
  getStyle: computed('height', function() {
    return {
      height: get(this, 'height'),
    };
  }),
  resize: function(e) {
    const $self = this.element;
    const $appContent = [...$$('main > div')][0];
    if ($appContent) {
      const rect = $self.getBoundingClientRect();
      const $footer = [...$$('footer[role="contentinfo"]')][0];
      const space = rect.top + $footer.clientHeight;
      const height = e.detail.height - space;
      this.set('height', Math.max(0, height));
      this.updateItems();
      this.updateScrollPosition();
    }
  },
});
