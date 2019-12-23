/* eslint no-control-regex: "off" */
import Yadda from 'yadda';
import YAML from 'js-yaml';
export default function(nspace, dict = new Yadda.Dictionary()) {
  dict
    .define('json', /([^\u0000]*)/, function(val, cb) {
      cb(null, JSON.parse(val));
    })
    .define('yaml', /([^\u0000]*)/, function(val, cb) {
      cb(null, YAML.safeLoad(val));
    })
    .define('model', /(\w+)/, function(model, cb) {
      switch (model) {
        case 'datacenter':
        case 'datacenters':
        case 'dcs':
          model = 'dc';
          break;
        case 'services':
          model = 'service';
          break;
        case 'nodes':
          model = 'node';
          break;
        case 'kvs':
          model = 'kv';
          break;
        case 'acls':
          model = 'acl';
          break;
        case 'sessions':
          model = 'session';
          break;
        case 'intentions':
          model = 'intention';
          break;
      }
      cb(null, model);
    })
    .define('number', /(\d+)/, Yadda.converters.integer);
  if (typeof nspace !== 'undefined' && nspace !== '') {
    dict.define('url', /([^\u0000]*)/, function(val, cb) {
      val = `/~${nspace}${val}`;
      cb(null, val);
    });
  }
  return dict;
}
