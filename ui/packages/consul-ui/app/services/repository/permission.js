import RepositoryService from 'consul-ui/services/repository';
import { inject as service } from '@ember/service';
import { tracked } from '@glimmer/tracking';

const modelName = 'permission';
export default class PermissionService extends RepositoryService {
  @service('env') env;
  @service('can') _can;
  // move this to the store
  @tracked permissions = [];

  getModelName() {
    return modelName;
  }

  has(permission) {
    const keys = Object.keys(permission);
    return this.permissions.some(item => {
      return keys.every(key => item[key] === permission[key]) && item.Allow === true;
    });
  }

  can(can) {
    return this._can.can(can);
  }

  generate(resource, action, segment) {
    const req = {
      Resource: resource,
      Access: action,
    };
    return req;
  }

  async findAll(dc, nspace) {
    const perms = [
      {
        Resource: 'operator',
        Access: 'write',
      },
      {
        Resource: 'service',
        Access: 'read',
      },
      {
        Resource: 'node',
        Access: 'read',
      },
      {
        Resource: 'session',
        Access: 'read',
      },
      {
        Resource: 'session',
        Access: 'write',
      },
      {
        Resource: 'key',
        Access: 'read',
      },
      {
        Resource: 'key',
        Access: 'write',
      },
      {
        Resource: 'intention',
        Access: 'read',
      },
      {
        Resource: 'intention',
        Access: 'write',
      },
      {
        Resource: 'acl',
        Access: 'read',
      },
      {
        Resource: 'acl',
        Access: 'write',
      },
    ];
    if (!this.env.var('CONSUL_ACLS_ENABLED')) {
      this.permissions = perms.map(item => {
        return {
          ...item,
          Allow: true,
        };
      });
    } else {
      this.permissions = await this.store
        .authorize('nspace', { dc: dc, ns: nspace, permissions: perms })
        .catch(function(e) {
          return [];
        });
    }
    return this.permissions;
  }
}
