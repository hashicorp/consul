export default function(visitable, attribute, submitable, deletable, cancelable) {
  return {
    visit: visitable(['/:dc/kv/:kv/edit', '/:dc/kv/create'], function(str) {
      // this will encode the parts of the key path but means you can no longer
      // visit with path parts containing slashes
      return str
        .split('/')
        .map(encodeURIComponent)
        .join('/');
    }),
    ...submitable({}, 'main'),
    ...cancelable(),
    ...deletable(),
    session: {
      ID: attribute('data-test-session', '[data-test-session]'),
      ...deletable({}, '[data-test-session]'),
    },
  };
}
