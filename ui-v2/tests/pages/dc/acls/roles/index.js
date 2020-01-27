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
    visit: visitable('/:dc/acls/roles'),
    roles: collection(
      '[data-test-tabular-row]',
      deletable({
        name: attribute('data-test-role', '[data-test-role]'),
        description: text('[data-test-description]'),
        policy: text('[data-test-policy].policy', { multiple: true }),
        serviceIdentity: text('[data-test-policy].policy-service-identity', { multiple: true }),
        actions: clickable('label'),
      })
    ),
    filter: filter,
  });
}
