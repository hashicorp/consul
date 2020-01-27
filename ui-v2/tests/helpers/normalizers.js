export const createPolicies = function(item) {
  return item.Policies.map(function(item) {
    return {
      template: '',
      ...item,
    };
  }).concat(
    item.ServiceIdentities.map(function(item) {
      const policy = {
        template: 'service-identity',
        Name: item.ServiceName,
      };
      if (typeof item.Datacenters !== 'undefined') {
        policy.Datacenters = item.Datacenters;
      }
      return policy;
    })
  );
};
