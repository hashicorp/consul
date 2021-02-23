import RepositoryService from 'consul-ui/services/repository';
import { get } from '@ember/object';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/token';
import statusFactory from 'consul-ui/utils/acls-status';
import isValidServerErrorFactory from 'consul-ui/utils/http/acl/is-valid-server-error';
import dataSource from 'consul-ui/decorators/data-source';

const isValidServerError = isValidServerErrorFactory();
const status = statusFactory(isValidServerError, Promise);
const MODEL_NAME = 'token';

export default class TokenService extends RepositoryService {
  getModelName() {
    return MODEL_NAME;
  }

  getPrimaryKey() {
    return PRIMARY_KEY;
  }

  getSlugKey() {
    return SLUG_KEY;
  }

  status(obj) {
    return status(obj);
  }

  @dataSource('/:ns/:dc/token/self/:secret')
  self(params) {
    // TODO: Does this need ns passing through?
    return this.store
      .self(this.getModelName(), {
        secret: params.secret,
        dc: params.dc,
      })
      .catch(e => {
        // If we get this 500 RPC error, it means we are a legacy ACL cluster
        // set AccessorID to null - which for the frontend means legacy mode
        if (isValidServerError(e)) {
          return {
            AccessorID: null,
            SecretID: params.secret,
          };
        }
        return Promise.reject(e);
      });
  }

  clone(item) {
    return this.store.clone(this.getModelName(), get(item, PRIMARY_KEY));
  }

  findByPolicy(params) {
    return this.store.query(this.getModelName(), {
      policy: params.id,
      dc: params.dc,
      ns: params.ns,
    });
  }

  findByRole(params) {
    return this.store.query(this.getModelName(), {
      role: params.id,
      dc: params.dc,
      ns: params.ns,
    });
  }
}
