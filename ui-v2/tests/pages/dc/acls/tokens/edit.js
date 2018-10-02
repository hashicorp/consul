export default function(visitable, submitable, deletable, cancelable, clickable, attribute, collection) {
  return submitable(
    cancelable(
      deletable(
        {
          visit: visitable(['/:dc/acls/tokens/:token', '/:dc/acls/tokens/create']),
          policies: collection(
            '[data-test-tabular-row]',
          ),
        },
        'form > div'
      )
    )
  );
}
