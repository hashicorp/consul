import Controller from '@ember/controller';
import WithEventSource from 'consul-ui/mixins/with-event-source';
export default Controller.extend(WithEventSource, {
  queryParams: {
    filterBy: {
      as: 'action',
    },
    search: {
      as: 'filter',
      replace: true,
    },
  },
  actions: {
    route: function() {
      this.send(...arguments);
    },
  },
});
