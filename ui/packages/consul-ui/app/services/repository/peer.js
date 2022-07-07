import RepositoryService from 'consul-ui/services/repository';
import dataSource from 'consul-ui/decorators/data-source';

export default class PeerService extends RepositoryService {

  getModelName() {
    return 'peer';
  }

  @dataSource('/:partition/:ns/:dc/peers')
  async fetchAll({ dc, ns, partition }, { uri }, request) {
    return (await request`
      GET /v1/peerings

      ${{
        partition,
      }}
    `)(
      (headers, body, cache) => {
        return {
          meta: {
            version: 2,
            interval: 10000,
            uri: uri,
          },
          body: body.map(item => {
            return cache(
              {
                ...item,
                Datacenter: dc,
                Partition: partition,
              },
              uri => uri`peer:///${partition}/${ns}/${dc}/peer/${item.Name}`
            );
          })
        };
      });
  }

  @dataSource('/:partition/:ns/:dc/peer/:name')
  async fetchOne({partition, ns, dc, name}, { uri }, request) {
    if (name === '') {
      return this.create({
        Datacenter: dc,
        Namespace: '',
        Partition: partition,
      });
    }
    return (await request`
      GET /v1/peering/${name}

      ${{
        partition,
      }}
    `)((headers, body, cache) => {
        return {
          meta: {
            version: 2,
            interval: 10000,
            uri: uri,
          },
          body: cache(
            {
              ...body,
              Datacenter: dc,
              Partition: partition,
            },
            uri => uri`peer:///${partition}/${ns}/${dc}/peer/${body.Name}`
          )
        };
    });
  }

  async remove(item, request) {
    // soft delete
    // we just return the item we want to delete
    // but mark it as DELETING ourselves as the request is successfull
    // and we don't have blocking queries here to get immediate updates
    return (await request`
      DELETE /v1/peering/${item.Name}
    `)((headers, body, cache) => {
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
              State: 'DELETING'
            },
            uri => uri`peer:///${partition}/${ns}/${dc}/peer/${item.Name}`
          )
        };
    });
  }
}
