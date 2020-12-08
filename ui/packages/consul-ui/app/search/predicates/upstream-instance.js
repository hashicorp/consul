export default () => term => item => {
  const lowerTerm = term.toLowerCase();
  return Object.entries(item)
    .filter(([key, value]) => key !== 'DestinationType')
    .some(
      ([key, value]) =>
        value
          .toString()
          .toLowerCase()
          .indexOf(lowerTerm) !== -1
    );
};
