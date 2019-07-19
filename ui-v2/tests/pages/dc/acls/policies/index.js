export default function(
  visitable,
  deletable,
  creatable,
  clickable,
  attribute,
  collection,
  text,
  filter
) {
  return creatable({
    visit: visitable('/:dc/acls/policies'),
    policies: collection(
      '[data-test-tabular-row]',
      deletable({
        name: attribute('data-test-policy', '[data-test-policy]'),
        description: text('[data-test-description]'),
        policy: clickable('a'),
        actions: clickable('label'),
      })
    ),
    filter: filter,
  });
}
