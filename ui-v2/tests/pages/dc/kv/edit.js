export default function(visitable, submitable, deletable, cancelable) {
  return submitable(
    cancelable(
      deletable({
        visit: visitable(['/:dc/kv/:kv/edit', '/:dc/kv/create'], function(str) {
          // this will encode the parts of the key path but means you can no longer
          // visit with path parts containing slashes
          return str
            .split('/')
            .map(encodeURIComponent)
            .join('/');
        }),
        session: deletable({}, '[data-test-session]'),
      })
    )
  );
}
