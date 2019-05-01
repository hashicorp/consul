export default function(visitable, deletable, creatable, clickable, attribute, collection, filter) {
  return creatable({
    visit: visitable('/:dc/acls/roles'),
    roles: collection(
      '[data-test-tabular-row]',
      deletable({
        name: attribute('data-test-role', '[data-test-role]'),
        policy: clickable('a'),
        actions: clickable('label'),
      })
    ),
    filter: filter,
  });
}
