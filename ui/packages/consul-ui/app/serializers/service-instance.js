import Serializer from './application';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/service-instance';

export default class ServiceInstanceSerializer extends Serializer {
  primaryKey = PRIMARY_KEY;
  slugKey = SLUG_KEY;

  hash = JSON.stringify;

  extractUid(item) {
    return this.hash([
      item.Partition || 'default',
      item.Namespace || 'default',
      item.Datacenter,
      item.Node.Node,
      item.Service.ID,
    ]);
  }

  transformHasManyResponseFromNode(node, item, checks) {
    const serviceChecks = checks[item.ID] || [];
    const statuses = serviceChecks.reduce(
      (prev, item) => {
        switch (item.Status) {
          case 'passing':
            prev.ChecksPassing.push(item);
            break;
          case 'warning':
            prev.ChecksWarning.push(item);
            break;
          case 'critical':
            prev.ChecksCritical.push(item);
            break;
        }
        return prev;
      },
      {
        ChecksPassing: [],
        ChecksWarning: [],
        ChecksCritical: [],
      }
    );
    const instance = {
      ...statuses,
      Service: item,
      Checks: serviceChecks,
      Node: {
        Datacenter: node.Datacenter,
        Namespace: node.Namespace,
        Partition: node.Partition,
        ID: node.ID,
        Node: node.Node,
        Address: node.Address,
        TaggedAddresses: node.TaggedAddresses,
        Meta: node.Meta,
      },
    };

    instance.uid = this.extractUid(instance);
    return instance;
  }

  respondForQuery(respond, query) {
    const body = super.respondForQuery(cb => {
      return respond((headers, body) => {
        if (body.length === 0) {
          const e = new Error();
          e.errors = [
            {
              status: '404',
              title: 'Not found',
            },
          ];
          throw e;
        }
        body.forEach(item => {
          item.Datacenter = query.dc;
          item.Namespace = query.ns || 'default';
          item.Partition = query.partition || 'default';
          item.uid = this.extractUid(item);
        });
        return cb(headers, body);
      });
    }, query);
    return body;
  }

  respondForQueryRecord(respond, query) {
    return super.respondForQueryRecord(cb => {
      return respond((headers, body) => {
        body.forEach(item => {
          item.Datacenter = query.dc;
          item.Namespace = query.ns || 'default';
          item.Partition = query.partition || 'default';
          item.uid = this.extractUid(item);
        });
        body = body.find(function(item) {
          return item.Node.Node === query.node && item.Service.ID === query.serviceId;
        });
        if (typeof body === 'undefined') {
          const e = new Error();
          e.errors = [
            {
              status: '404',
              title: 'Not found',
            },
          ];
          throw e;
        }
        body.Namespace = body.Service.Namespace;
        body.Partition = body.Service.Partition;
        return cb(headers, body);
      });
    }, query);
  }
}
