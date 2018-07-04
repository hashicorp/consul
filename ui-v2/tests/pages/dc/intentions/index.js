export default function(visitable, deletable, creatable, clickable, attribute, collection, filter) {
  return creatable({
    visit: visitable('/:dc/intentions'),
    intentions: collection(
      '[data-test-tabular-row]',
      deletable({
        source: attribute('data-test-intention-source', '[data-test-intention-source]'),
        destination: attribute(
          'data-test-intention-destination',
          '[data-test-intention-destination]'
        ),
        action: attribute('data-test-intention-action', '[data-test-intention-action]'),
        intention: clickable('a'),
        actions: clickable('label'),
      })
    ),
    filter: filter,
  });
}
