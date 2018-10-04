export default function(
  visitable,
  submitable,
  deletable,
  cancelable,
  clickable,
  attribute,
  collection,
  withPolicyForm
) {
  console.log(submitable({}));
  return submitable(
    cancelable(
      deletable(
        {
          visit: visitable(['/:dc/acls/tokens/:token', '/:dc/acls/tokens/create']),
          newPolicy: clickable('[data-test-new-policy]'),
          policyForm: submitable(
            cancelable({}, '[data-test-policy-form]'),
            '[data-test-policy-form]'
          ),
          policies: collection(
            '[data-test-tabular-row]',
            deletable(
              {
                expand: clickable('label'),
              },
              '+ tr'
            )
          ),
        },
        'form > div'
      )
    )
  );
}
