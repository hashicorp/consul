const hcl = require('micro-hcl');

module.exports = (babel) => {
  const transpile = function(template) {
    const str = template.get('quasi').get('quasis').map(item => item.node.value.cooked).join('');
    template.replaceWithSourceString(JSON.stringify(hcl.parse(str)));
  }
  return {
    visitor: {
      TaggedTemplateExpression: function(path) {
        const tag = path.get('tag');
        if(tag.isIdentifier({name: 'hcl'})) {
          transpile(path)
        };
      }
    }
  }
}
