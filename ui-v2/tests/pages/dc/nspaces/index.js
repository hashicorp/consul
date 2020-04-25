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
    visit: visitable('/:dc/namespaces'),
    nspaces: collection(
      '[data-test-tabular-row]',
      deletable({
        action: attribute('data-test-nspace-action', '[data-test-nspace-action]'),
        description: text('[data-test-description]'),
        nspace: clickable('a'),
        actions: clickable('label'),
      })
    ),
    filter: filter,
  });
}
