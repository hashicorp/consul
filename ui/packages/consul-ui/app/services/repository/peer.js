/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import RepositoryService from 'consul-ui/services/repository';
import dataSource from 'consul-ui/decorators/data-source';
import { inject as service } from '@ember/service';

function normalizePeerPayload(peerPayload, dc, partition) {
  const {
    StreamStatus: { LastHeartbeat, LastReceive, LastSend, ImportedServices, ExportedServices },
  } = peerPayload;

  return {
    ...peerPayload,
    LastHeartbeat,
    LastReceive,
    LastSend,
    ImportedServices,
    ExportedServices,
    Datacenter: dc,
    Partition: partition,
  };
}
export default class PeerService extends RepositoryService {
  @service store;

  getModelName() {
    return 'peer';
  }

  @dataSource('/:partition/:ns/:ds/exported-services/:name')
  async fetchExportedServices({ dc, ns, partition, name }, configuration, request) {
    return (
      await request`
      GET /v1/internal/ui/exported-services

      ${{
        peer: name,
      }}
    `
    )((headers, body, cache) => {
      const serviceSerializer = this.store.serializerFor('service');

      return this.store.push(
        serviceSerializer.createJSONApiDocumentFromServicesPayload(headers, body, dc)
      );
    });
  }

  @dataSource('/:partition/:ns/:dc/peering/token-for/:name/:externalAddresses')
  async fetchToken({ dc, ns, partition, name, externalAddresses }, configuration, request) {
    const ServerExternalAddresses =
      externalAddresses?.length > 0 ? externalAddresses.split(',') : [];

    return (
      await request`
      POST /v1/peering/token

      ${{
        PeerName: name,
        Partition: partition || undefined,
        ServerExternalAddresses,
      }}
    `
    )((headers, body, cache) => body);
  }

  @dataSource('/:partition/:ns/:dc/peers')
  async fetchAll({ dc, ns, partition }, { uri }, request) {
    return (
      await request`
      GET /v1/peerings

      ${{
        partition,
      }}
    `
    )((headers, body, cache) => {
      return {
        meta: {
          version: 2,
          interval: 10000,
          uri: uri,
        },
        body: body.map((item) => {
          return cache(
            normalizePeerPayload(item, dc, partition),
            (uri) => uri`peer:///${partition}/${ns}/${dc}/peer/${item.Name}`
          );
        }),
      };
    });
  }

  @dataSource('/:partition/:ns/:dc/peer-generate/')
  @dataSource('/:partition/:ns/:dc/peer-initiate/')
  @dataSource('/:partition/:ns/:dc/peer/:name')
  async fetchOne({ partition, ns, dc, name }, { uri }, request) {
    if (typeof name === 'undefined' || name === '') {
      const item = this.create({
        Datacenter: dc,
        Namespace: '',
        Partition: partition,
      });
      item.meta = { cacheControl: 'no-store' };
      return item;
    }
    return (
      await request`
      GET /v1/peering/${name}

      ${{
        partition,
      }}
    `
    )((headers, body, cache) => {
      // we can't easily use fragments as we are working around the serializer
      // layer
      const { StreamStatus } = body;

      if (StreamStatus) {
        if (StreamStatus.LastHeartbeat) {
          StreamStatus.LastHeartbeat = new Date(StreamStatus.LastHeartbeat);
        }
        if (StreamStatus.LastReceive) {
          StreamStatus.LastReceive = new Date(StreamStatus.LastReceive);
        }
        if (StreamStatus.LastSend) {
          StreamStatus.LastSend = new Date(StreamStatus.LastSend);
        }
      }

      return {
        meta: {
          version: 2,
          interval: 10000,
          uri: uri,
        },
        body: cache(
          normalizePeerPayload(body, dc, partition),
          (uri) => uri`peer:///${partition}/${ns}/${dc}/peer/${body.Name}`
        ),
      };
    });
  }

  async persist(item, request) {
    // mark it as ESTABLISHING ourselves as the request is successful
    // and we don't have blocking queries here to get immediate updates
    return (
      await request`
      POST /v1/peering/establish

      ${{
        PeerName: item.Name,
        PeeringToken: item.PeeringToken,
        Partition: item.Partition || undefined,
      }}
    `
    )((headers, body, cache) => {
      const partition = item.Partition;
      const ns = item.Namespace;
      const dc = item.Datacenter;
      return {
        meta: {
          version: 2,
        },
        body: cache(
          {
            ...item,
            State: 'ESTABLISHING',
          },
          (uri) => uri`peer:///${partition}/${ns}/${dc}/peer/${item.Name}`
        ),
      };
    });
  }

  async remove(item, request) {
    // soft delete
    // we just return the item we want to delete
    // but mark it as DELETING ourselves as the request is successful
    // and we don't have blocking queries here to get immediate updates
    return (
      await request`
      DELETE /v1/peering/${item.Name}
    `
    )((headers, body, cache) => {
      const partition = item.Partition;
      const ns = item.Namespace;
      const dc = item.Datacenter;
      return {
        meta: {
          version: 2,
        },
        body: cache(
          {
            ...item,
            State: 'DELETING',
          },
          (uri) => uri`peer:///${partition}/${ns}/${dc}/peer/${item.Name}`
        ),
      };
    });
  }
}
