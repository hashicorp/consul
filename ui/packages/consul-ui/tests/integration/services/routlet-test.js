import { moduleFor, test } from 'ember-qunit';
moduleFor('service:routlet', 'Integration | Routlet', {
  // Specify the other units that are required for this test.
  integration: true,
});
test('outletFor works', function(assert) {
  const routlet = this.subject();
  routlet.addOutlet('application', {
    name: 'application'
  });
  routlet.addRoute('dc', {});
  routlet.addOutlet('dc', {
    name: 'dc'
  });
  routlet.addRoute('dc.services', {});
  routlet.addOutlet('dc.services', {
    name: 'dc.services'
  });
  routlet.addRoute('dc.services.instances', {});

  let actual = routlet.outletFor('dc.services');
  let expected = 'dc';
  assert.equal(actual.name, expected);

  actual = routlet.outletFor('dc');
  expected = 'application';
  assert.equal(actual.name, expected);

  actual = routlet.outletFor('application');
  expected = undefined;
  assert.equal(actual, expected);
});
