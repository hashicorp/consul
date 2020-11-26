export default {
  ID: (item, value) => item.ID.toLowerCase().indexOf(value.toLowerCase()) !== -1,
  Name: (item, value) => item.Name.toLowerCase().indexOf(value.toLowerCase()) !== -1,
};
