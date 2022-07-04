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
}
