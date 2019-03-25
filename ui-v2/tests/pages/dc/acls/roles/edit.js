export default function(
  visitable,
  submitable,
  deletable,
  cancelable,
  clickable,
  attribute,
  collection
) {
  return submitable(
    cancelable(
      deletable({
        visit: visitable(['/:dc/acls/roles/:role', '/:dc/acls/roles/create']),
        tokens: collection(
          '[data-test-tokens] [data-test-tabular-row]',
          deletable({
            id: attribute('data-test-token', '[data-test-token]'),
            token: clickable('a'),
          })
        ),
        // TODO: Also see tokens/edit, these should get injected
        newPolicy: clickable('[data-test-new-policy]'),
        policyForm: submitable(
          cancelable({}, '[data-test-policy-form]'),
          '[data-test-policy-form]'
        ),
        policies: collection(
          '[data-test-policies] [data-test-tabular-row]',
          deletable(
            {
              expand: clickable('label'),
            },
            '+ tr'
          )
        ),
      })
    )
  );
}
