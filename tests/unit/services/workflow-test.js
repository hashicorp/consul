import { moduleFor } from 'ember-qunit';
import test from 'ember-sinon-qunit/test-support/test';

moduleFor('service:workflow', 'Unit | Service | workflow', {
  // Specify the other units that are required for this test.
  // needs: ['service:foo']
});

// Replace this with your real tests.
test('it exists', function(assert) {
  let service = this.subject();
  assert.ok(service);
});
test('execute calls hashCallback using resolve', function(assert) {
  const expected = true;
  const subject = this.subject();

  const hashCallback = this.stub();
  const hash = {
    expected: expected,
  };
  hashCallback.returns(hash);

  const resolve = this.stub(subject, 'resolve');
  resolve.returns(Promise.resolve(hash));

  const actual = subject.execute(hashCallback);
  assert.ok(hashCallback.calledOnce);
  assert.ok(resolve.calledOnce);
  assert.ok(resolve.calledWith(hash));
  return actual.then(function(hash) {
    assert.equal(hash.expected, expected);
  });
});
