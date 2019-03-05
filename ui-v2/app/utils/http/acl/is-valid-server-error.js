// very specific error check just for one specific ACL case
// likely to be reused at a later date, so lets use the specific
// case we need right now as default
const UNKNOWN_METHOD_ERROR = "rpc error making call: rpc: can't find method ACL";
export default function(response = UNKNOWN_METHOD_ERROR) {
  return function(e) {
    if (e && e.errors && e.errors[0] && e.errors[0].detail) {
      return e.errors[0].detail.indexOf(response) !== -1;
    }
    return false;
  };
}
