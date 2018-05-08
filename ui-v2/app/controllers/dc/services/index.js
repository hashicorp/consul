import Controller from '@ember/controller';
import { get, computed } from '@ember/object';
import WithHealthFiltering from 'consul-ui/mixins/with-health-filtering';
const max = function(arr, prop) {
  return arr.reduce(function(prev, item) {
    return Math.max(prev, get(item, prop));
  }, 0);
};
const chunk = function(str, size) {
  const num = Math.ceil(str.length / size);
  const chunks = new Array(num);
  for (let i = 0, o = 0; i < num; ++i, o += size) {
    chunks[i] = str.substr(o, size);
  }
  return chunks;
};
const width = function(num) {
  const str = num.toString();
  const len = str.length;
  const commas = chunk(str, 3).length - 1;
  const w = commas * 4 + len * 10;
  return `width: ${w}px`.htmlSafe();
};
export default Controller.extend(WithHealthFiltering, {
  filter: function(item, { s = '', status = '' }) {
    return (
      get(item, 'Name')
        .toLowerCase()
        .indexOf(s.toLowerCase()) !== -1 && item.hasStatus(status)
    );
  },
  maxPassing: computed('items', function() {
    return max(get(this, 'items'), 'ChecksPassing');
  }),
  maxWarning: computed('items', function() {
    return max(get(this, 'items'), 'ChecksWarning');
  }),
  maxCritical: computed('items', function() {
    return max(get(this, 'items'), 'ChecksCritical');
  }),
  passingWidth: computed('maxPassing', function() {
    return width(get(this, 'maxPassing'));
  }),
  warningWidth: computed('maxWarning', function() {
    return width(get(this, 'maxWarning'));
  }),
  criticalWidth: computed('maxCritical', function() {
    return width(get(this, 'maxCritical'));
  }),
});
