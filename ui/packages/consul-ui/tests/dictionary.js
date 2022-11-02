/* eslint no-control-regex: "off" */
import Yadda from 'yadda';
import YAML from 'js-yaml';
import { env } from '../env';
export default (utils) =>
  (annotations, nspace, dict = new Yadda.Dictionary()) => {
    dict
      .define('pageObject', /(\S+)/, function (path, cb) {
        const $el = utils.find(path);
        cb(null, $el);
      })
      .define('model', /(\w+)/, function (model, cb) {
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
      .define('number', /(\d+)/, Yadda.converters.integer)
      .define('json', /([^\u0000]*)/, function (val, cb) {
        // replace any instance of @namespace in the string
        val = val.replace(
          /@namespace/g,
          typeof nspace === 'undefined' || nspace === '' ? 'default' : nspace
        );
        cb(null, JSON.parse(val));
      })
      .define('yaml', /([^\u0000]*)/, function (val, cb) {
        // sometimes we need to always force a namespace queryParam
        // mainly for DELETEs
        if (env('CONSUL_NSPACES_ENABLED')) {
          val = val.replace(/ns=@!namespace/g, `ns=${nspace || 'default'}`);
          val = val.replace(/Namespace: @!namespace/g, `Namespace: ${nspace || 'default'}`);
        } else {
          val = val.replace(/&ns=@!namespace/g, '');
          val = val.replace(/&ns=\*/g, '');
          val = val.replace(/- \/v1\/namespaces/g, '');
          val = val.replace(/Namespace: @!namespace/g, '');
        }
        if (typeof nspace === 'undefined' || nspace === '') {
          val = val.replace(/Namespace: @namespace/g, '').replace(/&ns=@namespace/g, '');
        }
        // replace any other instance of @namespace in the string
        val = val.replace(
          /@namespace/g,
          typeof nspace === 'undefined' || nspace === '' ? 'default' : nspace
        );
        cb(null, YAML.load(val));
      })
      .define('endpoint', /([^\u0000]*)/, function (val, cb) {
        // if is @namespace is !important, always replace with namespace
        // or if its undefined or empty then use default
        if (env('CONSUL_NSPACES_ENABLED')) {
          val = val.replace(/ns=@!namespace/g, `ns=${nspace || 'default'}`);
        } else {
          val = val.replace(/&ns=@!namespace/g, '');
          val = val.replace(/&ns=\*/g, '');
        }
        // for endpoints if namespace isn't specified it should
        // never add the ns= unless its !important...
        if (typeof nspace !== 'undefined' && nspace !== '') {
          val = val.replace(/ns=@namespace/g, `ns=${nspace}`);
        } else {
          val = val
            .replace(/&ns=@namespace/g, '')
            .replace(/ns=@namespace&/g, '')
            .replace(/ns=@namespace/g, '');
        }
        cb(null, val);
      });
    if (typeof nspace !== 'undefined' && nspace !== '') {
      dict.define('url', /([^\u0000]*)/, function (val, cb) {
        val = `/~${nspace}${val}`;
        cb(null, val);
      });
    }
    return dict;
  };
