import { helper } from '@ember/component/helper';

const EXTERNAL_SOURCE_ICON_MAP = {
  kubernetes: 'kubernetes-color',
  terraform: 'terraform-color',
  nomad: 'nomad-color',
  consul: 'consul-color',
  'consul-api-gateway': 'consul-color',
  vault: 'vault',
  jwt: 'jwt-color',
  aws: 'aws-color',
  lambda: 'aws-lambda-color',
};

function externalSourceIconMap([icon]) {
  return EXTERNAL_SOURCE_ICON_MAP[icon];
}

export default helper(externalSourceIconMap);
