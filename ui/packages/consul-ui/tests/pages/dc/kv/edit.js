export default function(visitable, attribute, present, submitable, deletable, cancelable) {
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
    kv: {
      Key: attribute('data-test-kv-key', '[data-test-kv-key]')
    },
    session: {
      warning: present('[data-test-session-warning]'),
      ID: attribute('data-test-session', '[data-test-session]'),
      ...deletable({}, '[data-test-session]'),
    },
  };
}
