export default () => (term) => (item) => {
  const source = item.SourceName.toLowerCase();
  const destination = item.DestinationName.toLowerCase();
  const allLabel = 'All Services (*)'.toLowerCase();
  const lowerTerm = term.toLowerCase();
  return (
    source.indexOf(lowerTerm) !== -1 ||
    destination.indexOf(lowerTerm) !== -1 ||
    (source === '*' && allLabel.indexOf(lowerTerm) !== -1) ||
    (destination === '*' && allLabel.indexOf(lowerTerm) !== -1)
  );
}
