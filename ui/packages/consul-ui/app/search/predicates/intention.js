const allLabel = 'All Services (*)';
export default {
  SourceName: item =>
    [item.SourceName, item.SourceName === '*' ? allLabel : undefined].filter(item => Boolean),
  DestinationName: item =>
    [item.SourceName, item.DestinationName === '*' ? allLabel : undefined].filter(item => Boolean),
};
