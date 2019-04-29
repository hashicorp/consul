import { moduleForComponent, test } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('templated-anchor', 'Integration | Component | templated anchor', {
  integration: true,
});

test('it renders', function(assert) {
  [
    {
      href: 'http://localhost/?={{Name}}/{{ID}}',
      vars: {
        Name: 'name',
        ID: 'id',
      },
      result: 'http://localhost/?=name/id',
    },
    {
      href: 'http://localhost/?={{Name}}/{{ID}}',
      vars: {
        Name: '{{Name}}',
        ID: '{{ID}}',
      },
      result: 'http://localhost/?={{Name}}/{{ID}}',
    },
    {
      href: 'http://localhost/?={{deep.Name}}/{{deep.ID}}',
      vars: {
        deep: {
          Name: '{{Name}}',
          ID: '{{ID}}',
        },
      },
      result: 'http://localhost/?={{Name}}/{{ID}}',
    },
    {
      href: 'http://localhost/?={{}}/{{}}',
      vars: {
        Name: 'name',
        ID: 'id',
      },
      result: 'http://localhost/?={{}}/{{}}',
    },
    {
      href: 'http://localhost/?={{Service_Name}}/{{Meta-Key}}',
      vars: {
        Service_Name: 'name',
        ['Meta-Key']: 'id',
      },
      result: 'http://localhost/?=name/id',
    },
    {
      href: 'http://localhost/?={{Service_Name}}/{{Meta-Key}}',
      vars: {
        WrongPropertyName: 'name',
        ['Meta-Key']: 'id',
      },
      result: 'http://localhost/?=/id',
    },
    {
      href: 'http://localhost/?={{.Name}}',
      vars: {
        ['.Name']: 'name',
      },
      result: 'http://localhost/?=',
    },
    {
      href: 'http://localhost/?={{.}}',
      vars: {
        ['.']: 'name',
      },
      result: 'http://localhost/?=',
    },
    {
      href: 'http://localhost/?={{deep..Name}}',
      vars: {
        deep: {
          Name: 'Name',
          ID: 'ID',
        },
      },
      result: 'http://localhost/?=',
    },
  ].forEach(item => {
    this.set('item', item);
    this.render(hbs`
        {{#templated-anchor href=item.href vars=item.vars}}
          Dashboard link
        {{/templated-anchor}}
      `);
    assert.equal(
      this.$()
        .find('a')
        .attr('href'),
      item.result
    );
  });
});
