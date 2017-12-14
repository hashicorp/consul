from grpc_tools import protoc

protoc.main(('', '-I../protos', '--python_out=.', '--grpc_python_out=.',
             '../protos/command_line.proto'))
