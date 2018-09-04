export default function(visitable, submitable, deletable, cancelable, clickable) {
  return submitable(
    cancelable(
      deletable({
        visit: visitable(['/:dc/acls/tokens/:token', '/:dc/acls/tokens/create']),
      })
    )
  );
}
