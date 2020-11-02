import { get, set } from '@ember/object';

import kv from 'consul-ui/forms/kv';
import acl from 'consul-ui/forms/acl';
import token from 'consul-ui/forms/token';
import policy from 'consul-ui/forms/policy';
import role from 'consul-ui/forms/role';
import intention from 'consul-ui/forms/intention';
import nspace from 'consul-ui/forms/nspace';

export function initialize(application) {
  // Service-less injection using private properties at a per-project level
  const FormBuilder = application.resolveRegistration('service:form');
  const forms = {
    kv: kv,
    acl: acl,
    token: token,
    policy: policy,
    role: role,
    intention: intention,
    nspace: nspace,
  };
  FormBuilder.reopen({
    form: function(name) {
      let form = get(this.forms, name);
      if (!form) {
        form = set(this.forms, name, forms[name](this));
        // only do special things for our new things for the moment
        if (name === 'role' || name === 'policy') {
          let repo = get(this, name);
          // In the grand 'ember native class conversion' it seems like EmberObject
          // no longer proxy through to content if you don't specify a property
          // so here we manually check for the content property if the function we
          // are looking for doesn't exist
          if (typeof repo.create !== 'function') {
            repo = repo.content;
          }
          form.clear(function(obj) {
            return repo.create(obj);
          });
          form.submit(function(obj) {
            return repo.persist(obj);
          });
        }
      }
      return form;
    },
  });
}

export default {
  initialize,
};
