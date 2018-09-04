export default function(visitable, submitable, deletable, cancelable, clickable) {
  return submitable(
    cancelable(
      deletable({
        visit: visitable(['/:dc/acls/policies/:policy', '/:dc/acls/policies/create']),
      })
    )
  );
}
