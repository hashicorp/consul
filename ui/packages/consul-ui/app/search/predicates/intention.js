export default {
  SourceName: (item, value) => item.SourceName.toLowerCase().indexOf(value.toLowerCase()) !== -1,
  DestinationName: (item, value) =>
    item.DestinationName.toLowerCase().indexOf(value.toLowerCase()) !== -1,
};
