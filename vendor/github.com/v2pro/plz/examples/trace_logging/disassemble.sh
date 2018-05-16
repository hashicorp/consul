# https://github.com/golang/go/issues/9337
go build -tags release -gcflags="-l -l -l -l -l -l -m" main.go
go tool objdump -s main.trace_should_be_optimized_away main
go tool objdump -s main.trace_call_should_combine_the_error_checking main