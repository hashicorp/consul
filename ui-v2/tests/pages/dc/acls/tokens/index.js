export default function(visitable, deletable, creatable, clickable, attribute, collection, filter) {
  return creatable({
    visit: visitable('/:dc/acls/tokens'),
    tokens: collection(
      '[data-test-tabular-row]',
      deletable({
        id: attribute('data-test-token', '[data-test-token]'),
        token: clickable('a'),
        actions: clickable('label'),
        use: clickable('[data-test-use]'),
        confirmUse: clickable('button.type-delete'),
      })
    ),
    filter: filter,
  });
}
