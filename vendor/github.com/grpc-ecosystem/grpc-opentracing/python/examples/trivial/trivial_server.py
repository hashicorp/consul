from __future__ import print_function

import time
import argparse

import grpc
from concurrent import futures
from jaeger_client import Config

from grpc_opentracing import open_tracing_server_interceptor
from grpc_opentracing.grpcext import intercept_server

import command_line_pb2

_ONE_DAY_IN_SECONDS = 60 * 60 * 24


class CommandLine(command_line_pb2.CommandLineServicer):

    def Echo(self, request, context):
        return command_line_pb2.CommandResponse(text=request.text)


def serve():
    parser = argparse.ArgumentParser()
    parser.add_argument(
        '--log_payloads',
        action='store_true',
        help='log request/response objects to open-tracing spans')
    args = parser.parse_args()

    config = Config(
        config={
            'sampler': {
                'type': 'const',
                'param': 1,
            },
            'logging': True,
        },
        service_name='trivial-server')
    tracer = config.initialize_tracer()
    tracer_interceptor = open_tracing_server_interceptor(
        tracer, log_payloads=args.log_payloads)
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    server = intercept_server(server, tracer_interceptor)

    command_line_pb2.add_CommandLineServicer_to_server(CommandLine(), server)
    server.add_insecure_port('[::]:50051')
    server.start()
    try:
        while True:
            time.sleep(_ONE_DAY_IN_SECONDS)
    except KeyboardInterrupt:
        server.stop(0)

    time.sleep(2)
    tracer.close()
    time.sleep(2)


if __name__ == '__main__':
    serve()
