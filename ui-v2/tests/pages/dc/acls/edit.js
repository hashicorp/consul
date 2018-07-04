export default function(visitable, submitable, deletable, triggerable) {
  return submitable(
    deletable({
      visit: visitable(['/:dc/acls/:acl', '/:dc/acls/create']),
    })
  );
}
