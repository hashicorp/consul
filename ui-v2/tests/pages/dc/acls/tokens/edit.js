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
      deletable(
        {
          visit: visitable(['/:dc/acls/tokens/:token', '/:dc/acls/tokens/create']),
          use: clickable('[data-test-use]'),
          confirmUse: clickable('button.type-delete'),
          // TODO: Also see tokens/edit, these should get injected
          newPolicy: clickable('[for="new-policy-toggle"]'),
          newRole: clickable('[for="new-role-toggle"]'),
          policyForm: submitable(
            cancelable({}, '[data-test-policy-form]'),
            '[data-test-policy-form]'
          ),
          roleForm: submitable(
            cancelable(
              {
                newPolicy: clickable('[data-test-create-policy]'),
                policyForm: submitable({}, '[data-test-role-form]'),
              },
              '[data-test-role-form]'
            ),
            '[data-test-role-form]'
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
          roles: collection(
            '[data-test-roles] [data-test-tabular-row]',
            deletable({
              actions: clickable('label'),
            })
          ),
        },
        'form > div'
      )
    )
  );
}
