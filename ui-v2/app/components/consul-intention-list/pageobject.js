export default (collection, clickable, attribute, deletable) => () => {
  return collection('.consul-intention-list [data-test-tabular-row]', {
    source: attribute('data-test-intention-source', '[data-test-intention-source]'),
    destination: attribute('data-test-intention-destination', '[data-test-intention-destination]'),
    action: attribute('data-test-intention-action', '[data-test-intention-action]'),
    intention: clickable('a'),
    actions: clickable('label'),
    ...deletable(),
  });
};
