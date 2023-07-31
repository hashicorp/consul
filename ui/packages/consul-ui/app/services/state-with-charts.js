import StateService from 'consul-ui/services/state';

import validate from 'consul-ui/machines/validate.xstate';
import _boolean from 'consul-ui/machines/boolean.xstate';

export default class ChartedStateService extends StateService {
  stateCharts = {
    validate: validate,
    boolean: _boolean,
  };
}
