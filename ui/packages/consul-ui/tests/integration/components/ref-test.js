import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import { hbs } from 'ember-cli-htmlbars';

module('Integration | Component | ref', function (hooks) {
  setupRenderingTest(hooks);

  test('it renders', async function (assert) {
    // Set any properties with this.set('myProperty', 'value');
    // Handle any actions with this.set('myAction', function(val) { ... });
    const componentAction = function () {};
    // yield the action in the component, optionally changing the name
    // {{ yield (hash
    //   publicAction=(action 'componentAction')
    // )}}
    const _yield = {
      publicAction: componentAction,
    };
    this.set('api', _yield);
    await render(hbs`<Ref @target={{this}} @name="api" @value={{api}} />`);
    // the value is now available on the target
    // in most cases `this` i.e. the scope of the template (component/controller)
    assert.deepEqual(this.api, _yield);

    assert.equal(this.element.textContent.trim(), '');

    // // Template block usage:
    // await render(hbs`
    //   <Ref></Ref>
    // `);

    // assert.equal(this.element.textContent.trim(), '');
  });
});
