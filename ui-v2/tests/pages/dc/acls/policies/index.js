export default function(visitable, deletable, creatable, clickable, attribute, collection, filter) {
  return creatable({
    visit: visitable('/:dc/acls/policies'),
    policies: collection(
      '[data-test-tabular-row]',
      deletable({
        name: attribute('data-test-policy', '[data-test-policy]'),
        policy: clickable('a'),
        actions: clickable('label'),
      })
    ),
    filter: filter,
  });
}
