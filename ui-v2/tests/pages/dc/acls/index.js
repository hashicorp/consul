export default function(visitable, deletable, creatable, clickable, attribute, collection, filter) {
  return creatable({
    visit: visitable('/:dc/acls'),
    acls: collection(
      '[data-test-tabular-row]',
      deletable({
        name: attribute('data-test-acl', '[data-test-acl]'),
        acl: clickable('a'),
        actions: clickable('label'),
        use: clickable('[data-test-use]'),
        confirmUse: clickable('button.type-delete'),
      })
    ),
    filter: filter,
  });
}
