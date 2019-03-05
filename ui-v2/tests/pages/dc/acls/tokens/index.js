export default function(
  visitable,
  submitable,
  deletable,
  creatable,
  clickable,
  attribute,
  collection,
  text,
  filter
) {
  return submitable(
    creatable({
      visit: visitable('/:dc/acls/tokens'),
      update: text('[data-test-notification-update]'),
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
    })
  );
}
