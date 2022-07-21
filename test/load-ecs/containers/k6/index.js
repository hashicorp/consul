const { execSync } = require('child_process')

exports.handler = async (event) => {
  execSync('k6 run loadtest.js', { encoding: 'utf8', stdio: 'inherit' })
}
