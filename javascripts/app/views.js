
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
    //
    // We use the same template as we do for the services
    // array and have a simple conditional to display the nested
    // individual service resource.
    //
    templateName: 'service'
})


//
// Nodes
//

App.NodesView = Ember.View.extend({
    templateName: 'nodes'
})

App.NodesShowView = Ember.View.extend({
    //
    // We use the same template as we do for the nodes
    // array and have a simple conditional to display the nested
    // individual node resource.
    //
    templateName: 'node'
})



App.KvListView = Ember.View.extend({
    templateName: 'kv'
})
