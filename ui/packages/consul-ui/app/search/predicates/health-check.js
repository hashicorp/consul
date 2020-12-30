const asArray = function(arr) {
  return Array.isArray(arr) ? arr : arr.toArray();
};
export default {
  Name: (item, value) => item.Name,
  Node: (item, value) => item.Node,
  Service: (item, value) => item.ServiceName,
  CheckID: (item, value) => item.CheckID || '',
  ID: (item, value) => item.Service.ID || '',
  Notes: (item, value) => item.Notes,
  Output: (item, value) => item.Output,
  ServiceTags: (item, value) => asArray(item.ServiceTags || []),
};
