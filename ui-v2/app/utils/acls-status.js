// This is used by all acl routes to check whether
// acls are enabled on the server, and whether the user
// has a valid token
// Right now this is very acl specific, but is likely to be
// made a bit more less specific

export default function(isValidServerError, P = Promise) {
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
          if (e.errors && e.errors[0]) {
            switch (e.errors[0].status) {
              case '500':
                if (isValidServerError(e)) {
                  enable(true);
                  authorize(false);
                } else {
                  enable(false);
                  authorize(false);
                  return P.reject(e);
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
          }
          enable(false);
          authorize(false);
          throw e;
        })
        .then(function(res) {
          enable(true);
          authorize(true);
          return res;
        }),
    };
  };
}
