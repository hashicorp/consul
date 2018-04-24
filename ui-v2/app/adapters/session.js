import Adapter from './application';
import makeAttrable from 'consul-ui/utils/makeAttrable';

const PRIMARY_KEY = 'ID';
const DATACENTER_KEY = 'Datacenter';
export default Adapter.extend({
  urlForQuery: function(query, modelName) {
    const id = query.node;
    delete query.node;
    return this.appendURL('session/node', [id]);
  },
  urlForQueryRecord: function(query, modelName) {
    const id = query.key;
    delete query.key;
    return this.appendURL('session/info', [id]);
  },
  urlForDeleteRecord: function(id, modelName, snapshot) {
    const query = {
      dc: snapshot.attr(DATACENTER_KEY),
    };
    return this.appendURL('session/destroy', [id], query);
  },
  methodForRequest: function(params) {
    switch (params.requestType) {
      case 'deleteRecord':
        return 'PUT';
    }
    return this._super(...arguments);
  },
  handleResponse: function(status, headers, payload, requestData) {
    let response = payload;
    if (response === true) {
      const url = requestData.url.split('?')[0];
      const item = {
        [PRIMARY_KEY]: url
          .split('/')
          .splice(4)
          .join('/'),
        [DATACENTER_KEY]: '',
      };
      if (
        this.urlForDeleteRecord(item[PRIMARY_KEY], null, makeAttrable(item)).split('?')[0] === url
      ) {
        response = item;
      }
    }
    return this._super(status, headers, response, requestData);
  },
});
