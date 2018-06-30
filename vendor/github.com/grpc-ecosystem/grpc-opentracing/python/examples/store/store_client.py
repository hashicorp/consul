# A OpenTraced client for a Python service that implements the store interface.
from __future__ import print_function

import time
import argparse
from builtins import input, range

import grpc
from jaeger_client import Config

from grpc_opentracing import open_tracing_client_interceptor, \
                             SpanDecorator
from grpc_opentracing.grpcext import intercept_channel

import store_pb2


class CommandExecuter(object):

    def __init__(self, stub):
        self._stub = stub

    def _execute_rpc(self, method, via, timeout, request_or_iterator):
        if via == 'future':
            result = getattr(self._stub, method).future(request_or_iterator,
                                                        timeout)
            return result.result()
        elif via == 'with_call':
            return getattr(self._stub, method).with_call(request_or_iterator,
                                                         timeout)[0]
        else:
            return getattr(self._stub, method)(request_or_iterator, timeout)

    def do_stock_item(self, via, timeout, arguments):
        if len(arguments) != 1:
            print('must input a single item')
            return
        request = store_pb2.AddItemRequest(name=arguments[0])
        self._execute_rpc('AddItem', via, timeout, request)

    def do_stock_items(self, via, timeout, arguments):
        if not arguments:
            print('must input at least one item')
            return
        requests = [store_pb2.AddItemRequest(name=name) for name in arguments]
        self._execute_rpc('AddItems', via, timeout, iter(requests))

    def do_sell_item(self, via, timeout, arguments):
        if len(arguments) != 1:
            print('must input a single item')
            return
        request = store_pb2.RemoveItemRequest(name=arguments[0])
        response = self._execute_rpc('RemoveItem', via, timeout, request)
        if not response.was_successful:
            print('unable to sell')

    def do_sell_items(self, via, timeout, arguments):
        if not arguments:
            print('must input at least one item')
            return
        requests = [
            store_pb2.RemoveItemRequest(name=name) for name in arguments
        ]
        response = self._execute_rpc('RemoveItems', via, timeout,
                                     iter(requests))
        if not response.was_successful:
            print('unable to sell')

    def do_inventory(self, via, timeout, arguments):
        if arguments:
            print('inventory does not take any arguments')
            return
        if via != 'functor':
            print('inventory can only be called via functor')
            return
        request = store_pb2.Empty()
        result = self._execute_rpc('ListInventory', via, timeout, request)
        for query in result:
            print(query.name, '\t', query.count)

    def do_query_item(self, via, timeout, arguments):
        if len(arguments) != 1:
            print('must input a single item')
            return
        request = store_pb2.QueryItemRequest(name=arguments[0])
        query = self._execute_rpc('QueryQuantity', via, timeout, request)
        print(query.name, '\t', query.count)

    def do_query_items(self, via, timeout, arguments):
        if not arguments:
            print('must input at least one item')
            return
        if via != 'functor':
            print('query_items can only be called via functor')
            return
        requests = [store_pb2.QueryItemRequest(name=name) for name in arguments]
        result = self._execute_rpc('QueryQuantities', via, timeout,
                                   iter(requests))
        for query in result:
            print(query.name, '\t', query.count)


def execute_command(command_executer, command, arguments):
    via = 'functor'
    timeout = None
    for argument_index in range(0, len(arguments), 2):
        argument = arguments[argument_index]
        if argument == '--via' and argument_index + 1 < len(arguments):
            if via not in ('functor', 'with_call', 'future'):
                print('invalid --via option')
                return
            via = arguments[argument_index + 1]
        elif argument == '--timeout' and argument_index + 1 < len(arguments):
            timeout = float(arguments[argument_index + 1])
        else:
            arguments = arguments[argument_index:]
            break

    try:
        getattr(command_executer, 'do_' + command)(via, timeout, arguments)
    except AttributeError:
        print('unknown command: \"%s\"' % command)


INSTRUCTIONS = \
"""Enter commands to interact with the store service:

    stock_item     Stock a single item.
    stock_items    Stock one or more items.
    sell_item      Sell a single item.
    sell_items     Sell one or more items.
    inventory      List the store's inventory.
    query_item     Query the inventory for a single item.
    query_items    Query the inventory for one or more items.

You can also optionally provide a --via argument to instruct the RPC to be
initiated via either the functor, with_call, or future method; or provide a
--timeout argument to set a deadline for the RPC to be completed.

Example:
    > stock_item apple
    > stock_items --via future apple milk
    > inventory
    apple   2
    milk    1
"""


def read_and_execute(command_executer):
    print(INSTRUCTIONS)
    while True:
        try:
            line = input('> ')
            components = line.split()
            if not components:
                continue
            command = components[0]
            arguments = components[1:]
            execute_command(command_executer, command, arguments)
        except EOFError:
            break


class StoreSpanDecorator(SpanDecorator):

    def __call__(self, span, rpc_info):
        span.set_tag('grpc.method', rpc_info.full_method)
        span.set_tag('grpc.headers', str(rpc_info.metadata))
        span.set_tag('grpc.deadline', str(rpc_info.timeout))


def run():
    parser = argparse.ArgumentParser()
    parser.add_argument(
        '--log_payloads',
        action='store_true',
        help='log request/response objects to open-tracing spans')
    parser.add_argument(
        '--include_grpc_tags',
        action='store_true',
        help='set gRPC-specific tags on spans')
    args = parser.parse_args()

    config = Config(
        config={
            'sampler': {
                'type': 'const',
                'param': 1,
            },
            'logging': True,
        },
        service_name='store-client')
    tracer = config.initialize_tracer()
    span_decorator = None
    if args.include_grpc_tags:
        span_decorator = StoreSpanDecorator()
    tracer_interceptor = open_tracing_client_interceptor(
        tracer, log_payloads=args.log_payloads, span_decorator=span_decorator)
    channel = grpc.insecure_channel('localhost:50051')
    channel = intercept_channel(channel, tracer_interceptor)
    stub = store_pb2.StoreStub(channel)

    read_and_execute(CommandExecuter(stub))

    time.sleep(2)
    tracer.close()
    time.sleep(2)


if __name__ == '__main__':
    run()
