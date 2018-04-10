import Controller from '@ember/controller';
import WithHealthFiltering from 'consul-ui/mixins/with-health-filtering';
import ResizeAware from 'ember-resize/mixins/resize-aware';
import { get } from '@ember/object';
const $ = document.querySelectorAll.bind(document);
export default Controller.extend(ResizeAware, WithHealthFiltering, {
  setProperties: function() {
    this._super(...arguments);
    this.get('resizeService').on('didResize', event =>
      this.didResize(window.innerWidth, window.innerHeight, event)
    );
    setTimeout(() => {
      this.didResize(window.innerWidth, window.innerHeight);
    }, 0);
  },
  didResize(width, height, evt) {
    const $header = [...$('#wrapper > header')][0];
    const $footer = [...$('#wrapper > footer')][0];
    const $thead = [...$('#wrapper thead')][0];
    if($thead) {
      this.set('height', new Number(height - ($footer.clientHeight + 335)));
      this.set('width', new Number($thead.clientWidth));
    }
  },
  filter: function(item, { s = '', status = '' }) {
    return (
      get(item, 'Name')
        .toLowerCase()
        .indexOf(s.toLowerCase()) === 0 && item.hasStatus(status)
    );
  },
});
