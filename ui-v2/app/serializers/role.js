import Serializer from './application';
import { PRIMARY_KEY } from 'consul-ui/models/role';
const createTemplatedPolicies = function(item) {
  item.ServiceIdentities.forEach(function(item) {
    const policy = {
      Name: item.ServiceName,
      template: 'service-identity',
      Datacenters: item.Datacenters,
    };
    // if(typeof item.Datancenters !== 'undefined') {

    // }
    if (typeof item.Policies === 'undefined') {
      item.Policies = [];
    }
    item.Policies.push(policy);
  });
  return item;
};
export default Serializer.extend({
  primaryKey: PRIMARY_KEY,
  normalizeResponse: function(store, primaryModelClass, payload, id, requestType) {
    if (requestType === 'queryRecord') {
      payload = createTemplatedPolicies(payload);
    } else {
      payload = payload.map(createTemplatedPolicies);
    }
    return this._super(...arguments);
  },
});
