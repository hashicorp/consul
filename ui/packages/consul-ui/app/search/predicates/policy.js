export default {
  Name: (item, value) => item.Name.toLowerCase().indexOf(value.toLowerCase()) !== -1,
  Description: (item, value) => item.Description.toLowerCase().indexOf(value.toLowerCase()) !== -1,
};
