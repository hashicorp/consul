require 'tempfile'
require_relative 'common'

namespace :test do
  desc 'Run linting on the repository'
  task :lint do
    # enable-gc is required because with a full linting process we may finish workers memory
    # fast is used temporarily for a faster CI
    sh 'gometalinter --deadline 60s --fast --enable-gc --errors --vendor ./opentracing ./tracer ./contrib/...'
  end

  desc 'Test all packages'
  task :all do
    sh 'go test ./opentracing ./tracer ./contrib/...'
  end

  desc 'Test all packages with -race flag'
  task :race do
    sh 'go test -race ./opentracing ./tracer ./contrib/...'
  end

  desc 'Run test coverage'
  task :coverage do
    # collect global profiles in this file
    sh "echo \"mode: count\" > #{Tasks::Common::COVERAGE_FILE}"

    # for each package collect and append the profile
    Tasks::Common.get_go_packages.each do |pkg|
      begin
        f = Tempfile.new('profile')
        # run code coverage
        sh "go test -short -covermode=count -coverprofile=#{f.path} #{pkg}"
        sh "cat #{f.path} | tail -n +2 >> #{Tasks::Common::COVERAGE_FILE}"
      ensure
        File.delete(f)
      end
    end

    sh "go tool cover -func #{Tasks::Common::COVERAGE_FILE}"
  end
end
