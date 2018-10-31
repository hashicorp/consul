// This is used by all acl routes to check whether
// acls are enabled on the server, and whether the user
// has a valid token
// Right now this is very acl specific, but is likely to be
// made a bit more less specific

// This is repeated from repository/token, I'd rather it was repeated and not imported
// for the moment at least
const UNKNOWN_METHOD_ERROR = "rpc error making call: rpc: can't find method ACL";
export default function(P = Promise) {
  return function(obj) {
    const propName = Object.keys(obj)[0];
    const p = obj[propName];
    let authorize;
    let enable;
    return {
      isAuthorized: new P(function(resolve) {
        authorize = function(bool) {
          resolve(bool);
        };
      }),
      isEnabled: new P(function(resolve) {
        enable = function(bool) {
          resolve(bool);
        };
      }),
      [propName]: p
        .catch(function(e) {
          switch (e.errors[0].status) {
            case '500':
              if (e.errors[0].detail.indexOf(UNKNOWN_METHOD_ERROR) === 0) {
                enable(true);
                authorize(false);
              }
              break;
            case '403':
              enable(true);
              authorize(false);
              break;
            case '401':
              enable(false);
              authorize(false);
              break;
            default:
              enable(false);
              authorize(false);
              throw e;
          }
          return [];
        })
        .then(function(res) {
          enable(true);
          authorize(true);
          return res;
        }),
    };
  };
}
