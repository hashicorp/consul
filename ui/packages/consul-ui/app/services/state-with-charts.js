import StateService from 'consul-ui/services/state';

import validate from 'consul-ui/machines/validate.xstate';

export default class ChartedStateService extends StateService {
  stateCharts = {
    'validate': validate
  };
}

