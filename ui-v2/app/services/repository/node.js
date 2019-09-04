import RepositoryService from 'consul-ui/services/repository';
import { inject as service } from '@ember/service';
import { get } from '@ember/object';

const modelName = 'node';
export default RepositoryService.extend({
  coordinates: service('repository/coordinate'),
  getModelName: function() {
    return modelName;
  },
  findByLeader: function(dc) {
    const query = {
      dc: dc,
    };
    return get(this, 'store').queryLeader(this.getModelName(), query);
  },
});
