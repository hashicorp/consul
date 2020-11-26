const allLabel = 'All Services (*)'.toLowerCase();
export default {
  SourceName: (item, value) =>
    item.SourceName.toLowerCase().indexOf(value.toLowerCase()) !== -1 ||
    (item.SourceName === '*' && allLabel.indexOf(value.toLowerCase()) !== -1),
  DestinationName: (item, value) =>
    item.DestinationName.toLowerCase().indexOf(value.toLowerCase()) !== -1 ||
    (item.DestinationName === '*' && allLabel.indexOf(value.toLowerCase()) !== -1),
};
