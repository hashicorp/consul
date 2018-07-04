export default function(visitable, submitable, deletable, cancelable) {
  return submitable(
    cancelable(
      deletable({
        visit: visitable(['/:dc/kv/:kv/edit', '/:dc/kv/create'], str => str),
      })
    )
  );
}
