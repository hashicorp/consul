import Fragment from 'ember-data-model-fragments/fragment';
import { array } from 'ember-data-model-fragments/attributes';
import { attr } from '@ember-data/model';
import { computed } from '@ember/object';
import { replace, nullValue } from 'consul-ui/decorators/replace';

export const schema = {
  Status: {
    allowedValues: ['passing', 'warning', 'critical'],
  },
  Type: {
    allowedValues: ['serf', 'script', 'http', 'tcp', 'ttl', 'docker', 'grpc', 'alias'],
  },
};

export default class HealthCheck extends Fragment {
  @attr('string') Name;
  @attr('string') CheckID;
  // an empty Type means its the Consul serf Check
  @replace('', 'serf') @attr('string') Type;
  @attr('string') Status;
  @attr('string') Notes;
  @attr('string') Output;
  @attr('string') ServiceName;
  @attr('string') ServiceID;
  @attr('string') Node;
  @nullValue([]) @array('string') ServiceTags;
  @attr() Definition; // {}

  // Exposed is only set correct if this Check is accessed via instance.MeshChecks
  // essentially this is a lazy MeshHealthCheckModel
  @attr('boolean') Exposed;

  @computed('ServiceID')
  get Kind() {
    return this.ServiceID === '' ? 'node' : 'service';
  }

  @computed('Type')
  get Exposable() {
    return ['http', 'grpc'].includes(this.Type);
  }
}
