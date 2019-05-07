export default function(visitable, submitable, deletable, cancelable, policySelector, tokenList) {
  return {
    visit: visitable(['/:dc/acls/roles/:role', '/:dc/acls/roles/create']),
    ...submitable({}, 'form > div'),
    ...cancelable({}, 'form > div'),
    ...deletable({}, 'form > div'),
    policies: policySelector(''),
    tokens: tokenList(),
  };
}
