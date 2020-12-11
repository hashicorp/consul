const asArray = function(arr) {
  return Array.isArray(arr) ? arr : arr.toArray();
};
export default {
  Name: (item, value) => {
    return item.Name.toLowerCase().indexOf(value.toLowerCase()) !== -1;
  },
  Node: (item, value) => {
    return item.Node.toLowerCase().indexOf(value.toLowerCase()) !== -1;
  },
  Service: (item, value) => {
    const lower = value.toLowerCase();
    return (
      item.ServiceName.toLowerCase().indexOf(lower) !== -1 ||
      item.ServiceID.toLowerCase().indexOf(lower) !== -1
    );
  },
  CheckID: (item, value) => (item.CheckID || '').toLowerCase().indexOf(value.toLowerCase()) !== -1,
  Notes: (item, value) =>
    item.Notes.toString()
      .toLowerCase()
      .indexOf(value.toLowerCase()) !== -1,
  Output: (item, value) =>
    item.Output.toString()
      .toLowerCase()
      .indexOf(value.toLowerCase()) !== -1,
  ServiceTags: (item, value) => {
    return asArray(item.ServiceTags || []).some(
      item => item.toLowerCase().indexOf(value.toLowerCase()) !== -1
    );
  },
};
