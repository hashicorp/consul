from __future__ import print_function

import time
import argparse

import grpc
from jaeger_client import Config

from grpc_opentracing import open_tracing_client_interceptor, ActiveSpanSource
from grpc_opentracing.grpcext import intercept_channel

import command_line_pb2


class FixedActiveSpanSource(ActiveSpanSource):

    def __init__(self):
        self.active_span = None

    def get_active_span(self):
        return self.active_span


def echo(tracer, active_span_source, stub):
    with tracer.start_span('command_line_client_span') as span:
        active_span_source.active_span = span
        response = stub.Echo(
            command_line_pb2.CommandRequest(text='Hello, hello'))
        print(response.text)


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
        service_name='integration-client')
    tracer = config.initialize_tracer()
    active_span_source = FixedActiveSpanSource()
    tracer_interceptor = open_tracing_client_interceptor(
        tracer,
        active_span_source=active_span_source,
        log_payloads=args.log_payloads)
    channel = grpc.insecure_channel('localhost:50051')
    channel = intercept_channel(channel, tracer_interceptor)
    stub = command_line_pb2.CommandLineStub(channel)

    echo(tracer, active_span_source, stub)

    time.sleep(2)
    tracer.close()
    time.sleep(2)


if __name__ == '__main__':
    run()
