import Component from '@ember/component';
import { computed, get } from '@ember/object';
import style from 'ember-computed-style';
export default Component.extend({
  classNames: ['healthchecked-resource'],
  attributeBindings: ['style'],
  style: style('gridRowEnd'),
  gridRowEnd: computed('UnhealthyChecks', function() {
    return {
      gridRow: `auto / span ${4 + (get(this, 'unhealthy.length') || 0)}`
    };
  })
});
