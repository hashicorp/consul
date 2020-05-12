import Controller from '@ember/controller';
import { get, computed } from '@ember/object';
import WithSearching from 'consul-ui/mixins/with-searching';

export default Controller.extend(WithSearching, {
  queryParams: {
    s: {
      as: 'filter',
      replace: true,
    },
  },
  init: function() {
    this.searchParams = {
      intention: 's',
    };
    this._super(...arguments);
  },
  searchable: computed('intentions', function() {
    return get(this, 'searchables.intention')
      .add(this.intentions)
      .search(get(this, this.searchParams.intention));
  }),
  actions: {
    route: function() {
      this.send(...arguments);
    },
  },
});
