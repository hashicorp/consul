import { inject as service } from '@ember/service';
import { computed, get } from '@ember/object';
import Component from 'ember-collection/components/ember-collection';
import PercentageColumns from 'ember-collection/layouts/percentage-columns';
import style from 'ember-computed-style';
import WithResizing from 'consul-ui/mixins/with-resizing';

export default Component.extend(WithResizing, {
  dom: service('dom'),
  tagName: 'div',
  attributeBindings: ['style'],
  height: 500,
  style: style('getStyle'),
  classNames: ['list-collection'],
  init: function() {
    this._super(...arguments);
    this.columns = [100];
  },
  didReceiveAttrs: function() {
    this._super(...arguments);
    this._cellLayout = this['cell-layout'] = new PercentageColumns(
      get(this, 'items.length'),
      get(this, 'columns'),
      get(this, 'cellHeight')
    );
  },
  getStyle: computed('height', function() {
    return {
      height: get(this, 'height'),
    };
  }),
  resize: function(e) {
    // TODO: This top part is very similar to resize in tabular-collection
    // see if it make sense to DRY out
    const dom = get(this, 'dom');
    const $appContent = dom.element('main > div');
    if ($appContent) {
      const border = 1;
      const rect = this.element.getBoundingClientRect();
      const $footer = dom.element('footer[role="contentinfo"]');
      const space = rect.top + $footer.clientHeight + border;
      const height = e.detail.height - space;
      this.set('height', Math.max(0, height));
      this.updateItems();
      this.updateScrollPosition();
    }
  },
  actions: {
    click: function(e) {
      return this.dom.clickFirstAnchor(e, '.list-collection > ul > li');
    },
  },
});
