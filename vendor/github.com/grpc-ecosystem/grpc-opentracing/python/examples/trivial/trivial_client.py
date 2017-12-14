from __future__ import print_function

import time
import argparse

import grpc
from jaeger_client import Config

from grpc_opentracing import open_tracing_client_interceptor
from grpc_opentracing.grpcext import intercept_channel

import command_line_pb2


def run():
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
        service_name='trivial-client')
    tracer = config.initialize_tracer()
    tracer_interceptor = open_tracing_client_interceptor(
        tracer, log_payloads=args.log_payloads)
    channel = grpc.insecure_channel('localhost:50051')
    channel = intercept_channel(channel, tracer_interceptor)
    stub = command_line_pb2.CommandLineStub(channel)
    response = stub.Echo(command_line_pb2.CommandRequest(text='Hello, hello'))
    print(response.text)

    time.sleep(2)
    tracer.close()
    time.sleep(2)


if __name__ == '__main__':
    run()
