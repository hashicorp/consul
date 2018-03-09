module Tasks
  module Common
    PROFILES = './profiles'
    TRACER_PACKAGE = 'github.com/DataDog/dd-trace-go/tracer'
    COVERAGE_FILE = 'code.cov'

    # returns a list of Go packages
    def self.get_go_packages
      `go list ./opentracing ./tracer ./contrib/...`.split("\n")
    end
  end
end
