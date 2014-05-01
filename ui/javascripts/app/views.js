
//
// DC
//

App.DcView = Ember.View.extend({
    templateName: 'dc',
    classNames: 'dropdowns',

    click: function(e){
        if ($(e.target).is('.dropdowns')){
          $('ul.dropdown-menu').hide();
        }
    }
})

//
// Services
//
App.ServicesView = Ember.View.extend({
    templateName: 'services',
})

App.ServicesShowView = Ember.View.extend({
    templateName: 'service'
})

App.ServicesLoadingView = Ember.View.extend({
    templateName: 'item/loading'
})

//
// Nodes
//

App.NodesView = Ember.View.extend({
    templateName: 'nodes'
})

App.NodesShowView = Ember.View.extend({
    templateName: 'node'
})

App.NodesLoadingView = Ember.View.extend({
    templateName: 'item/loading'
})

App.KvListView = Ember.View.extend({
    templateName: 'kv'
})
