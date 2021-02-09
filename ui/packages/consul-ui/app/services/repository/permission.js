import RepositoryService from 'consul-ui/services/repository';
import { inject as service } from '@ember/service';
import { tracked } from '@glimmer/tracking';

const modelName = 'permission';
export default class PermissionService extends RepositoryService {
  @service('env') env;
  @service('can') _can;
  // TODO: move this to the store, if we want it to use ember-data
  // currently this overwrites an inherited permissions service (this service)
  // which isn't ideal, but if the name of this changes be aware that we'd
  // probably have some circular dependency happening here
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
    if (typeof segment !== 'undefined') {
      req.Segment = segment;
    }
    return req;
  }

  async findBySlug(dc, nspace, model, segment) {
    let ability;
    try {
      ability = this._can.abilityFor(model);
    } catch (e) {
      return [];
    }
    const resources = ability.generateForSegment(segment.toString());
    if (!this.env.var('CONSUL_ACLS_ENABLED')) {
      return resources.map(item => {
        return {
          ...item,
          Allow: true,
        };
      });
    } else {
      return await this.store
        .authorize('nspace', { dc: dc, ns: nspace, permissions: resources })
        .catch(function(e) {
          return [];
        });
    }
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
