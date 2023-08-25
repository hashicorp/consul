const allLabel = 'All Services (*)';
export default {
  SourceName: (item) =>
    [item.SourceName, item.SourceName === '*' ? allLabel : undefined].filter(Boolean),
  DestinationName: (item) =>
    [item.DestinationName, item.DestinationName === '*' ? allLabel : undefined].filter(Boolean),
};
