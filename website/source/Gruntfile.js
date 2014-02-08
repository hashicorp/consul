module.exports = function(grunt) {

	// Configuration goes here
	grunt.initConfig({

  	concat: {
    	options: {
      		separator: ';'
    	},

    	site: {
    		src: 	[	
                'javascripts/app/app.js',
                'javascripts/app/util.js',
                'javascripts/app/home.js',
                'javascripts/app/nodes.js'
    					],

  			dest:  'javascripts/app/deploy/site.js'
    	},
		},

		uglify: {
      app: {
				files: {
					'javascripts/app/deploy/site.min.js': ['javascripts/app/deploy/site.js'] 
				}
			}
		},

		watch: {
		  js: {
		    files: 'javascripts/app/*.js',
		    tasks: ['concat', 'uglify']
		  }
		}

	});


	// Load plugins here
	grunt.loadNpmTasks('grunt-contrib-clean');
	grunt.loadNpmTasks('grunt-contrib-concat');
	grunt.loadNpmTasks('grunt-contrib-connect');
	grunt.loadNpmTasks('grunt-contrib-copy');
	grunt.loadNpmTasks('grunt-contrib-uglify');
	grunt.loadNpmTasks('grunt-contrib-watch');
	grunt.loadNpmTasks('grunt-recess');

  // JS distribution task.
  grunt.registerTask('dist-js', ['concat', 'uglify']);

  // Full distribution task.
  grunt.registerTask('dist', ['dist-js']);  

};