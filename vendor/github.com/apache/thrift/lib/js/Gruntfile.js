//To build dist/thrift.js, dist/thrift.min.js and doc/*
//run grunt at the command line in this directory.
//Prerequisites:
// Node Setup -   nodejs.org
// Grunt Setup -  npm install  //reads the ./package.json and installs project dependencies

module.exports = function(grunt) {
  'use strict';

  grunt.initConfig({
    pkg: grunt.file.readJSON('package.json'),
    concat: {
      options: {
        separator: ';'
      },
      dist: {
        src: ['src/**/*.js'],
        dest: 'dist/<%= pkg.name %>.js'
      }
    },
    jsdoc : {
        dist : {
            src: ['src/*.js', './README.md'],
            options: {
              destination: 'doc'
            }
        }
    },
    uglify: {
      options: {
        banner: '/*! <%= pkg.name %> <%= grunt.template.today("dd-mm-yyyy") %> */\n'
      },
      dist: {
        files: {
          'dist/<%= pkg.name %>.min.js': ['<%= concat.dist.dest %>']
        }
      }
    },
    shell: {
      InstallThriftJS: {
        command: 'mkdir test/build; mkdir test/build/js; mkdir test/build/js/lib; cp src/thrift.js test/build/js/thrift.js'
      },
      InstallThriftNodeJSDep: {
        command: 'cd ../..; npm install'
      },
      InstallTestLibs: {
        command: 'cd test; ant download_jslibs'
      },
      ThriftGen: {
        command: '../../compiler/cpp/thrift -gen js -gen js:node -o test ../../test/ThriftTest.thrift'
      },
      ThriftGenJQ: {
        command: '../../compiler/cpp/thrift -gen js:jquery -gen js:node -o test ../../test/ThriftTest.thrift'
      },
      ThriftGenDeepConstructor: {
        command: '../../compiler/cpp/thrift -gen js -o test ../../test/JsDeepConstructorTest.thrift'
      },
      ThriftGenDoubleConstants: {
        command: '../../compiler/cpp/thrift -gen js -o test ../../test/DoubleConstantsTest.thrift'
      },
      ThriftGenES6: {
        command: '../../compiler/cpp/thrift -gen js -gen js:es6 -o test ../../test/ThriftTest.thrift'
      },
      ThriftTestServer: {
        options: {
          async: true,
          execOptions: {
            cwd: "./test",
            env: {NODE_PATH: "../../nodejs/lib:../../../node_modules"}
          }
        },
        command: "node server_http.js",
      },
      ThriftTestServer_TLS: {
        options: {
          async: true,
          execOptions: {
            cwd: "./test",
            env: {NODE_PATH: "../../nodejs/lib:../../../node_modules"}
          }
        },
        command: "node server_https.js",
      },
    },
    qunit: {
      ThriftJS: {
        options: {
          urls: [
            'http://localhost:8088/test-nojq.html'
          ]
        }
      },
      ThriftJSJQ: {
        options: {
          urls: [
            'http://localhost:8088/test.html'
          ]
        }
      },
      ThriftJS_DoubleRendering: {
        options: {
          '--ignore-ssl-errors': true,
          urls: [
            'http://localhost:8088/test-double-rendering.html'
          ]
        }
      },
      ThriftWS: {
        options: {
          urls: [
            'http://localhost:8088/testws.html'
          ]
        }
      },
      ThriftJS_TLS: {
        options: {
          '--ignore-ssl-errors': true,
          urls: [
            'https://localhost:8089/test-nojq.html'
          ]
        }
      },
      ThriftJSJQ_TLS: {
        options: {
          '--ignore-ssl-errors': true,
          urls: [
            'https://localhost:8089/test.html'
          ]
        }
      },
      ThriftWS_TLS: {
        options: {
          '--ignore-ssl-errors': true,
          urls: [
            'https://localhost:8089/testws.html'
          ]
        }
      },
      ThriftDeepConstructor: {
        options: {
          urls: [
            'http://localhost:8088/test-deep-constructor.html'
          ]
        }
      },
      ThriftWSES6: {
        options: {
          urls: [
            'http://localhost:8088/test-es6.html'
          ]
        }
      }
    },
    jshint: {
      files: ['Gruntfile.js', 'src/**/*.js', 'test/*.js'],
      options: {
        // options here to override JSHint defaults
        globals: {
          jQuery: true,
          console: true,
          module: true,
          document: true
        }
      }
    },
  });

  grunt.loadNpmTasks('grunt-contrib-uglify');
  grunt.loadNpmTasks('grunt-contrib-jshint');
  grunt.loadNpmTasks('grunt-contrib-qunit');
  grunt.loadNpmTasks('grunt-contrib-concat');
  grunt.loadNpmTasks('grunt-jsdoc');
  grunt.loadNpmTasks('grunt-shell-spawn');

  grunt.registerTask('wait', 'Wait just one second for server to start', function () {
    var done = this.async();
    setTimeout(function() {
      done(true);
    }, 1000);
  });

  grunt.registerTask('test', ['jshint', 'shell:InstallThriftJS', 'shell:InstallThriftNodeJSDep', 'shell:ThriftGen',
                              'shell:InstallTestLibs',
                              'shell:ThriftTestServer', 'shell:ThriftTestServer_TLS',
                              'wait',
                              'shell:ThriftGenDeepConstructor', 'qunit:ThriftDeepConstructor',
                              'qunit:ThriftJS', 'qunit:ThriftJS_TLS',
                              'qunit:ThriftWS',
                              'shell:ThriftGenJQ', 'qunit:ThriftJSJQ', 'qunit:ThriftJSJQ_TLS',
                              'shell:ThriftGenES6', 'qunit:ThriftWSES6',
                              'shell:ThriftTestServer:kill', 'shell:ThriftTestServer_TLS:kill',
                             ]);
  grunt.registerTask('default', ['test', 'concat', 'uglify', 'jsdoc']);
};
