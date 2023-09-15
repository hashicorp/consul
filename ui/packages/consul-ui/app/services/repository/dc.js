/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Error from '@ember/error';
import { inject as service } from '@ember/service';
import RepositoryService from 'consul-ui/services/repository';
import dataSource from 'consul-ui/decorators/data-source';
import { HEADERS_DEFAULT_ACL_POLICY as DEFAULT_ACL_POLICY } from 'consul-ui/utils/http/consul';

const SECONDS = 1000;
const MODEL_NAME = 'dc';

const zero = {
  Total: 0,
  Passing: 0,
  Warning: 0,
  Critical: 0,
};
const aggregate = (prev, body, type) => {
  return body[type].reduce((prev, item) => {
    // for each Partitions, Namespaces
    ['Partition', 'Namespace'].forEach((bucket) => {
      // lazily initialize
      let obj = prev[bucket][item[bucket]];
      if (typeof obj === 'undefined') {
        obj = prev[bucket][item[bucket]] = {
          Name: item[bucket],
        };
      }
      if (typeof obj[type] === 'undefined') {
        obj[type] = {
          ...zero,
        };
      }
      //

      // accumulate
      obj[type].Total += item.Total;
      obj[type].Passing += item.Passing;
      obj[type].Warning += item.Warning;
      obj[type].Critical += item.Critical;
    });

    // also aggregate the Datacenter, without doubling up
    // for Partitions/Namespaces
    prev.Datacenter[type].Total += item.Total;
    prev.Datacenter[type].Passing += item.Passing;
    prev.Datacenter[type].Warning += item.Warning;
    prev.Datacenter[type].Critical += item.Critical;
    return prev;
  }, prev);
};

export default class DcService extends RepositoryService {
  @service('env') env;

  getModelName() {
    return MODEL_NAME;
  }

  @dataSource('/:partition/:ns/:dc/datacenters')
  async fetchAll({ partition, ns, dc }, { uri }, request) {
    const Local = this.env.var('CONSUL_DATACENTER_LOCAL');
    const Primary = this.env.var('CONSUL_DATACENTER_PRIMARY');
    return (
      await request`
      GET /v1/catalog/datacenters
      X-Request-ID: ${uri}
    `
    )((headers, body, cache) => {
      // TODO: Not sure nowadays whether we need to keep lowercasing everything
      // I vaguely remember when I last looked it was not needed for browsers anymore
      // but I also vaguely remember something about Pretender lowercasing things still
      // so if we can work around Pretender I think we can remove all the header lowercasing
      // For the moment we lowercase here so as to not effect the ember-data-flavoured-v1 fork
      const entry = Object.entries(headers).find(
        ([key, value]) => key.toLowerCase() === DEFAULT_ACL_POLICY.toLowerCase()
      );
      //
      const DefaultACLPolicy = entry[1] || 'allow';
      return {
        meta: {
          version: 2,
          uri: uri,
        },
        body: body.map((dc) => {
          return cache(
            {
              Name: dc,
              Datacenter: '',
              Local: dc === Local,
              Primary: dc === Primary,
              DefaultACLPolicy: DefaultACLPolicy,
            },
            (uri) => uri`${MODEL_NAME}:///${''}/${''}/${dc}/datacenter`
          );
        }),
      };
    });
  }

  @dataSource('/:partition/:ns/:dc/datacenter')
  async fetch({ partition, ns, dc }, { uri }, request) {
    return (
      await request`
      GET /v1/operator/autopilot/state?${{ dc }}
      X-Request-ID: ${uri}
    `
    )((headers, body, cache) => {
      // turn servers into an array instead of a map/object
      const servers = Object.values(body.Servers);
      const grouped = [];
      return {
        meta: {
          version: 2,
          uri: uri,
        },
        body: cache(
          {
            ...body,
            // all servers
            Servers: servers,
            RedundancyZones: Object.entries(body.RedundancyZones || {}).map(([key, value]) => {
              const zone = {
                ...value,
                Name: key,
                Healthy: true,
                // convert the string[] to Server[]
                Servers: value.Servers.reduce((prev, item) => {
                  const server = body.Servers[item];
                  // keep a record of things
                  grouped.push(server.ID);
                  prev.push(server);
                  return prev;
                }, []),
              };
              return zone;
            }),
            ReadReplicas: (body.ReadReplicas || []).map((item) => {
              // keep a record of things
              grouped.push(item);
              return body.Servers[item];
            }),
            Default: {
              Servers: servers.filter((item) => !grouped.includes(item.ID)),
            },
          },
          (uri) => uri`${MODEL_NAME}:///${''}/${''}/${dc}/datacenter`
        ),
      };
    });
  }

  @dataSource('/:partition/:ns/:dc/catalog/health')
  async fetchCatalogHealth({ partition, ns, dc }, { uri }, request) {
    return (
      await request`
      GET /v1/internal/ui/catalog-overview?${{ dc, stale: null }}
      X-Request-ID: ${uri}
    `
    )((headers, body, cache) => {
      // for each Services/Nodes/Checks aggregate
      const agg = ['Nodes', 'Services', 'Checks'].reduce(
        (prev, item) => aggregate(prev, body, item),
        {
          Datacenter: {
            Name: dc,
            Nodes: {
              ...zero,
            },
            Services: {
              ...zero,
            },
            Checks: {
              ...zero,
            },
          },
          Partition: {},
          Namespace: {},
        }
      );

      return {
        meta: {
          version: 2,
          uri: uri,
          interval: 30 * SECONDS,
        },
        body: {
          Datacenter: agg.Datacenter,
          Partitions: Object.values(agg.Partition),
          Namespaces: Object.values(agg.Namespace),
          ...body,
        },
      };
    });
  }

  @dataSource('/:partition/:ns/:dc/datacenter-cache/:name')
  async find(params) {
    const items = this.store.peekAll('dc');
    const item = items.findBy('Name', params.name);
    if (typeof item === 'undefined') {
      // TODO: We should use a HTTPError error here and remove all occurances of
      // the custom shaped ember-data error throughout the app
      const e = new Error('Page not found');
      e.status = '404';
      throw { errors: [e] };
    }
    return item;
  }
}
