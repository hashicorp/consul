import Controller from '@ember/controller';
import { get, computed } from '@ember/object';
import { htmlSafe } from '@ember/string';
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
  return commas * 4 + len * 10;
};
const widthDeclaration = function(num) {
  return htmlSafe(`width: ${num}px`);
};
export default Controller.extend(WithHealthFiltering, {
  filter: function(item, { s = '', status = '' }) {
    const term = s.toLowerCase();
    return (
      (get(item, 'Name')
        .toLowerCase()
        .indexOf(term) !== -1 ||
        (get(item, 'Tags') || []).some(function(item) {
          return item.toLowerCase().indexOf(term) !== -1;
        })) &&
      item.hasStatus(status)
    );
  },
  totalWidth: computed('{maxPassing,maxWarning,maxCritical}', function() {
    const PADDING = 32 * 3 + 13;
    return ['maxPassing', 'maxWarning', 'maxCritical'].reduce((prev, item) => {
      return prev + width(get(this, item));
    }, PADDING);
  }),
  thWidth: computed('totalWidth', function() {
    return widthDeclaration(get(this, 'totalWidth'));
  }),
  remainingWidth: computed('totalWidth', function() {
    return htmlSafe(`width: calc(50% - ${Math.round(get(this, 'totalWidth') / 2)}px)`);
  }),
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
    return widthDeclaration(width(get(this, 'maxPassing')));
  }),
  warningWidth: computed('maxWarning', function() {
    return widthDeclaration(width(get(this, 'maxWarning')));
  }),
  criticalWidth: computed('maxCritical', function() {
    return widthDeclaration(width(get(this, 'maxCritical')));
  }),
});
