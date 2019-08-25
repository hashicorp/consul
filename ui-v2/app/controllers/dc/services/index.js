import Controller from '@ember/controller';
import { get, computed } from '@ember/object';
import { htmlSafe } from '@ember/string';
import WithEventSource from 'consul-ui/mixins/with-event-source';
import WithSearching from 'consul-ui/mixins/with-searching';
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
export default Controller.extend(WithEventSource, WithSearching, {
  queryParams: {
    s: {
      as: 'filter',
    },
  },
  init: function() {
    this.searchParams = {
      service: 's',
    };
    this._super(...arguments);
  },
  searchable: computed('items.[]', function() {
    return get(this, 'searchables.service')
      .add(get(this, 'items'))
      .search(get(this, 'terms'));
  }),
  maxWidth: computed('{maxPassing,maxWarning,maxCritical}', function() {
    const PADDING = 32 * 3 + 13;
    return ['maxPassing', 'maxWarning', 'maxCritical'].reduce((prev, item) => {
      return prev + width(get(this, item));
    }, PADDING);
  }),
  totalWidth: computed('maxWidth', function() {
    return widthDeclaration(get(this, 'maxWidth'));
  }),
  remainingWidth: computed('maxWidth', function() {
    // maxWidth is the maximum width of the healthchecks column
    // there are currently 2 other columns so divide it by 2 and
    // take that off 50% (100% / number of fluid columns)
    // also we added a Type column which we've currently fixed to 100px
    // so again divide that by 2 and take it off each fluid column
    return htmlSafe(`width: calc(50% - ${Math.round(get(this, 'maxWidth') / 2)}px)`);
  }),
  maxPassing: computed('items.[]', function() {
    return max(get(this, 'items'), 'ChecksPassing');
  }),
  maxWarning: computed('items.[]', function() {
    return max(get(this, 'items'), 'ChecksWarning');
  }),
  maxCritical: computed('items.[]', function() {
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
