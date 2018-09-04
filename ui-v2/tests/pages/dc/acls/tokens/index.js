export default function(visitable, deletable, creatable, clickable, attribute, collection, filter) {
  return creatable({
    visit: visitable('/:dc/acls/tokens'),
    tokens: collection(
      '[data-test-tabular-row]',
      deletable({
        name: attribute('data-test-token', '[data-test-token]'),
        token: clickable('a'),
        actions: clickable('label'),
      })
    ),
    filter: filter,
  });
}
