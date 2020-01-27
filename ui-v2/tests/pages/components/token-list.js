export default (clickable, attribute, collection, deletable) => () => {
  return collection('[data-test-tokens] [data-test-tabular-row]', {
    id: attribute('data-test-token', '[data-test-token]'),
    token: clickable('a'),
    ...deletable(),
  });
};
