const directionify = arr => {
  return arr.reduce((prev, item) => prev.concat([`${item}:asc`, `${item}:desc`]), []);
};
export default () => key => {
  const comparables = directionify(['DestinationName']);
  return [comparables.find(item => item === key) || comparables[0]];
};
