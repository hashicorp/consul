desc 'Initialize the development environment'
task :init do
  sh 'go get -u github.com/golang/dep/cmd/dep'
  sh 'go get -u github.com/alecthomas/gometalinter'
  sh 'gometalinter --install'

  # TODO:bertrand remove this
  # It is only a short-term workaround, we should find a proper way to handle
  # multiple versions of the same dependency
  sh 'go get -d google.golang.org/grpc'
  gopath = ENV["GOPATH"].split(":")[0]
  sh "cd #{gopath}/src/google.golang.org/grpc/ && git checkout v1.5.2 && cd -"
  sh "go get -t -v ./contrib/..."
  sh "go get -v github.com/opentracing/opentracing-go"
end

namespace :vendors do
  desc "Update the vendors list"
  task :update do
    # download and update our vendors
    sh 'dep ensure'
  end
end
