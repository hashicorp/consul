export default function(visitable, submitable, deletable, clickable) {
  return submitable(
    deletable({
      visit: visitable(['/:dc/acls/:acl', '/:dc/acls/create']),
      use: clickable('[data-test-use]'),
      confirmUse: clickable('button.type-delete'),
    })
  );
}
