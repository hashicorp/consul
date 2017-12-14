"""Creates a simple service on top of gRPC for testing."""

from builtins import input, range

import grpc
from grpc.framework.foundation import logging_pool
from grpc_opentracing import grpcext

_SERIALIZE_REQUEST = lambda bytestring: bytestring
_DESERIALIZE_REQUEST = lambda bytestring: bytestring
_SERIALIZE_RESPONSE = lambda bytestring: bytestring
_DESERIALIZE_RESPONSE = lambda bytestring: bytestring

_UNARY_UNARY = '/test/UnaryUnary'
_UNARY_STREAM = '/test/UnaryStream'
_STREAM_UNARY = '/test/StreamUnary'
_STREAM_STREAM = '/test/StreamStream'

_STREAM_LENGTH = 5


class _MethodHandler(grpc.RpcMethodHandler):

    def __init__(self,
                 request_streaming=False,
                 response_streaming=False,
                 unary_unary=None,
                 unary_stream=None,
                 stream_unary=None,
                 stream_stream=None):
        self.request_streaming = request_streaming
        self.response_streaming = response_streaming
        self.request_deserializer = _DESERIALIZE_REQUEST
        self.response_serializer = _SERIALIZE_RESPONSE
        self.unary_unary = unary_unary
        self.unary_stream = unary_stream
        self.stream_unary = stream_unary
        self.stream_stream = stream_stream


class Handler(object):

    def __init__(self):
        self.invocation_metadata = None
        self.trailing_metadata = None

    def handle_unary_unary(self, request, servicer_context):
        if servicer_context is not None:
            self.invocation_metadata = servicer_context.invocation_metadata()
            if self.trailing_metadata is not None:
                servicer_context.set_trailing_metadata(self.trailing_metadata)
        return request

    def handle_unary_stream(self, request, servicer_context):
        if servicer_context is not None:
            self.invocation_metadata = servicer_context.invocation_metadata()
            if self.trailing_metadata is not None:
                servicer_context.set_trailing_metadata(self.trailing_metadata)
        for _ in range(_STREAM_LENGTH):
            yield request

    def handle_stream_unary(self, request_iterator, servicer_context):
        if servicer_context is not None:
            self.invocation_metadata = servicer_context.invocation_metadata()
            if self.trailing_metadata is not None:
                servicer_context.set_trailing_metadata(self.trailing_metadata)
        return b''.join(list(request_iterator))

    def handle_stream_stream(self, request_iterator, servicer_context):
        if servicer_context is not None:
            self.invocation_metadata = servicer_context.invocation_metadata()
            if self.trailing_metadata is not None:
                servicer_context.set_trailing_metadata(self.trailing_metadata)
        for request in request_iterator:
            yield request


def _set_error_code(servicer_context):
    servicer_context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
    servicer_context.set_details('ErroringHandler')


class ErroringHandler(Handler):

    def __init__(self):
        super(ErroringHandler, self).__init__()

    def handle_unary_unary(self, request, servicer_context):
        _set_error_code(servicer_context)
        return super(ErroringHandler, self).handle_unary_unary(request,
                                                               servicer_context)

    def handle_unary_stream(self, request, servicer_context):
        _set_error_code(servicer_context)
        return super(ErroringHandler, self).handle_unary_stream(
            request, servicer_context)

    def handle_stream_unary(self, request_iterator, servicer_context):
        _set_error_code(servicer_context)
        return super(ErroringHandler, self).handle_stream_unary(
            request_iterator, servicer_context)

    def handle_stream_stream(self, request_iterator, servicer_context):
        _set_error_code(servicer_context)
        return super(ErroringHandler, self).handle_stream_stream(
            request_iterator, servicer_context)


class ExceptionErroringHandler(Handler):

    def __init__(self):
        super(ExceptionErroringHandler, self).__init__()

    def handle_unary_unary(self, request, servicer_context):
        raise IndexError()

    def handle_unary_stream(self, request, servicer_context):
        raise IndexError()

    def handle_stream_unary(self, request_iterator, servicer_context):
        raise IndexError()

    def handle_stream_stream(self, request_iterator, servicer_context):
        raise IndexError()


class _GenericHandler(grpc.GenericRpcHandler):

    def __init__(self, handler):
        self._handler = handler

    def service(self, handler_call_details):
        method = handler_call_details.method
        if method == _UNARY_UNARY:
            return _MethodHandler(unary_unary=self._handler.handle_unary_unary)
        elif method == _UNARY_STREAM:
            return _MethodHandler(
                response_streaming=True,
                unary_stream=self._handler.handle_unary_stream)
        elif method == _STREAM_UNARY:
            return _MethodHandler(
                request_streaming=True,
                stream_unary=self._handler.handle_stream_unary)
        elif method == _STREAM_STREAM:
            return _MethodHandler(
                request_streaming=True,
                response_streaming=True,
                stream_stream=self._handler.handle_stream_stream)
        else:
            return None


class Service(object):

    def __init__(self,
                 client_interceptors,
                 server_interceptors,
                 handler=Handler()):
        self.handler = handler
        self._server_pool = logging_pool.pool(2)
        self._server = grpcext.intercept_server(
            grpc.server(self._server_pool), *server_interceptors)
        port = self._server.add_insecure_port('[::]:0')
        self._server.add_generic_rpc_handlers((_GenericHandler(self.handler),))
        self._server.start()
        self.channel = grpcext.intercept_channel(
            grpc.insecure_channel('localhost:%d' % port), *client_interceptors)

    @property
    def unary_unary_multi_callable(self):
        return self.channel.unary_unary(_UNARY_UNARY)

    @property
    def unary_stream_multi_callable(self):
        return self.channel.unary_stream(
            _UNARY_STREAM,
            request_serializer=_SERIALIZE_REQUEST,
            response_deserializer=_DESERIALIZE_RESPONSE)

    @property
    def stream_unary_multi_callable(self):
        return self.channel.stream_unary(
            _STREAM_UNARY,
            request_serializer=_SERIALIZE_REQUEST,
            response_deserializer=_DESERIALIZE_RESPONSE)

    @property
    def stream_stream_multi_callable(self):
        return self.channel.stream_stream(
            _STREAM_STREAM,
            request_serializer=_SERIALIZE_REQUEST,
            response_deserializer=_DESERIALIZE_RESPONSE)
