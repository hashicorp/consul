const asArray = function(arr) {
  return Array.isArray(arr) ? arr : arr.toArray();
};
export default {
  Name: (item, value) => {
    return item.Name.toLowerCase().indexOf(value.toLowerCase()) !== -1;
  },
  ID: (item, value) => (item.Service.ID || '').toLowerCase().indexOf(value.toLowerCase()) !== -1,
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
