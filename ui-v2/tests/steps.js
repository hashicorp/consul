/* eslint no-console: "off" */
import Inflector from 'ember-inflector';
import yadda from './helpers/yadda';
import utils from '@ember/test-helpers';
import getDictionary from '@hashicorp/ember-cli-api-double/dictionary';
import pages from 'consul-ui/tests/pages';
import api from 'consul-ui/tests/helpers/api';
import steps from './_steps';
// const dont = `( don't| shouldn't| can't)?`;
const pluralize = function(str) {
  return Inflector.inflector.pluralize(str);
};
export default function(assert) {
  const library = yadda.localisation.English.library(
    getDictionary(function(model, cb) {
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
    }, yadda)
  );
  return steps(assert, library, api, pages, utils, pluralize);
}
