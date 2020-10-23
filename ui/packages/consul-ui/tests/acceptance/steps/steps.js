import Inflector from 'ember-inflector';
import helpers from '@ember/test-helpers';
import $ from '-jquery';

import steps from 'consul-ui/tests/steps';
import pages from 'consul-ui/tests/pages';

import api from 'consul-ui/tests/helpers/api';

export default function({ assert, utils, library }) {
  return steps({
    assert,
    utils,
    library,
    pages,
    helpers,
    api,
    Inflector,
    $,
  });
}
