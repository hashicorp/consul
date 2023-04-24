module.exports = {
  test_page: 'tests/index.html?hidepassed',
  disable_watching: true,
  launch_in_ci: ['Chrome'],
  launch_in_dev: ['Chrome', 'Firefox', 'Safari'].includes(process.env.TESTEM_AUTOLAUNCH) ?
    [process.env.TESTEM_AUTOLAUNCH] : typeof process.env.TESTEM_AUTOLAUNCH === 'undefined' ? ['Chrome'] : [],
  browser_start_timeout: 120,
  browser_args: {
    Chrome: {
      ci: [
        // --no-sandbox is needed when running Chrome inside a container
        process.env.CI ? '--no-sandbox' : null,
        '--headless',
        '--disable-dev-shm-usage',
        '--disable-software-rasterizer',
        '--mute-audio',
        '--remote-debugging-port=0',
        '--window-size=1440,900',
      ].filter(Boolean),
    },
  },
};

// outputs XML reports for CI
if (process.env.EMBER_TEST_REPORT) {
  module.exports.report_file = process.env.EMBER_TEST_REPORT;
  module.exports.xunit_intermediate_output = true;
}

/*
 * ember-exam honors the `parallel` parameter in testem.js.
 * By default this value is 1 which means it only uses one client.
 * When this is set to -1 it uses the --split value of ember-exam.
 *
 * https://github.com/trentmwillis/ember-exam#split-test-parallelization
 * https://github.com/trentmwillis/ember-exam/issues/108
 */
if (process.env.EMBER_EXAM_PARALLEL) {
  module.exports.parallel = -1;
}
