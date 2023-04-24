import Service, { inject as service } from '@ember/service';
import { setProperties } from '@ember/object';

export default class HttpService extends Service {
  @service('client/http') client;

  @service('settings') settings;
  @service('repository/intention') intention;
  @service('repository/kv') kv;
  @service('repository/nspace') nspace;
  @service('repository/partition') partition;
  @service('repository/peer') peer;
  @service('repository/session') session;

  prepare(sink, data, instance) {
    return setProperties(instance, data);
  }

  // TODO: Currently we don't use the other properties here So dc, nspace and
  // partition, but confusingly they currently are in a different order to all
  // our @dataSource uris @dataSource uses /:partition/:nspace/:dc/thing whilst
  // here DataSink uses /:parition/:dc/:nspace/thing We should change DataSink
  // to also use a @dataSink decorator and make sure the order of the parameters
  // is the same throughout the app As it stands right now, if we do need to use
  // those parameters for DataSink it will be very easy to introduce a bug due
  // to this inconsistency
  persist(sink, instance) {
    const [, , , , model] = sink.split('/');
    const repo = this[model];
    return this.client.request(
      request => repo.persist(instance, request)
    );
  }

  remove(sink, instance) {
    const [, , , , model] = sink.split('/');
    const repo = this[model];
    return this.client.request(
      request => repo.remove(instance, request)
    );
  }
}
