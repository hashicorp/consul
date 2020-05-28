import Controller from '@ember/controller';

import WithEventSource from 'consul-ui/mixins/with-event-source';
export default Controller.extend(WithEventSource, {
  queryParams: {
    search: {
      as: 'filter',
    },
  },
});
