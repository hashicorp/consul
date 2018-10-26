export default function(visitable, submitable, deletable, cancelable, clickable, attribute, collection) {
  return submitable(
    cancelable(
      deletable({
        visit: visitable(['/:dc/acls/policies/:policy', '/:dc/acls/policies/create']),
        tokens: collection(
          '[data-test-tabular-row]',
          deletable({
            id: attribute('data-test-token', '[data-test-token]'),
            token: clickable('a'),
          })
        ),
      })
    )
  );
}
