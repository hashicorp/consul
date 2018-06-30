require_relative 'common'

namespace :collect do
  desc 'Run client benchmarks'
  task :benchmarks do
    # TODO: benchmarks must be done for different Tracer versions
    # so that we can retrieve the diff and return an exit code != 0
    Tasks::Common.get_go_packages.each do |pkg|
      sh "go test -run=NONE -bench=. #{pkg}"
    end
  end

  desc 'Run pprof to collect profiles'
  task :profiles do
    # initializes the folder to collect profiles
    FileUtils.mkdir_p 'profiles'
    filename = "#{Tasks::Common::PROFILES}/tracer"

    # generate a profile for the Tracer based on benchmarks
    sh %{
      go test -run=NONE -bench=.
      -cpuprofile=#{filename}-cpu.out
      -memprofile=#{filename}-mem.out
      -blockprofile=#{filename}-block.out
      #{Tasks::Common::TRACER_PACKAGE}
    }.gsub(/\s+/, ' ').strip

    # export profiles
    sh "go tool pprof -text -nodecount=10 -cum ./tracer.test #{filename}-cpu.out"
    sh "go tool pprof -text -nodecount=10 -cum -inuse_space ./tracer.test #{filename}-mem.out"
    sh "go tool pprof -text -nodecount=10 -cum ./tracer.test #{filename}-block.out"
  end
end
