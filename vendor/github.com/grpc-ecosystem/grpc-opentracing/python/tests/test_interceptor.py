import unittest

import grpc
from grpc_opentracing import grpcext

from _service import Service


class ClientInterceptor(grpcext.UnaryClientInterceptor,
                        grpcext.StreamClientInterceptor):

    def __init__(self):
        self.intercepted = False

    def intercept_unary(self, request, metadata, client_info, invoker):
        self.intercepted = True
        return invoker(request, metadata)

    def intercept_stream(self, request_or_iterator, metadata, client_info,
                         invoker):
        self.intercepted = True
        return invoker(request_or_iterator, metadata)


class ServerInterceptor(grpcext.UnaryServerInterceptor,
                        grpcext.StreamServerInterceptor):

    def __init__(self):
        self.intercepted = False

    def intercept_unary(self, request, servicer_context, server_info, handler):
        self.intercepted = True
        return handler(request, servicer_context)

    def intercept_stream(self, request_or_iterator, servicer_context,
                         server_info, handler):
        self.intercepted = True
        return handler(request_or_iterator, servicer_context)


class InterceptorTest(unittest.TestCase):
    """Test that RPC calls are intercepted."""

    def setUp(self):
        self._client_interceptor = ClientInterceptor()
        self._server_interceptor = ServerInterceptor()
        self._service = Service([self._client_interceptor],
                                [self._server_interceptor])

    def testUnaryUnaryInterception(self):
        multi_callable = self._service.unary_unary_multi_callable
        request = b'\x01'
        expected_response = self._service.handler.handle_unary_unary(request,
                                                                     None)
        response = multi_callable(request)

        self.assertEqual(response, expected_response)
        self.assertTrue(self._client_interceptor.intercepted)
        self.assertTrue(self._server_interceptor.intercepted)

    def testUnaryUnaryInterceptionWithCall(self):
        multi_callable = self._service.unary_unary_multi_callable
        request = b'\x01'
        expected_response = self._service.handler.handle_unary_unary(request,
                                                                     None)
        response, call = multi_callable.with_call(request)

        self.assertEqual(response, expected_response)
        self.assertIs(grpc.StatusCode.OK, call.code())
        self.assertTrue(self._client_interceptor.intercepted)
        self.assertTrue(self._server_interceptor.intercepted)

    def testUnaryUnaryInterceptionFuture(self):
        multi_callable = self._service.unary_unary_multi_callable
        request = b'\x01'
        expected_response = self._service.handler.handle_unary_unary(request,
                                                                     None)
        response = multi_callable.future(request).result()

        self.assertEqual(response, expected_response)
        self.assertTrue(self._client_interceptor.intercepted)
        self.assertTrue(self._server_interceptor.intercepted)

    def testUnaryStreamInterception(self):
        multi_callable = self._service.unary_stream_multi_callable
        request = b'\x01'
        expected_response = self._service.handler.handle_unary_stream(request,
                                                                      None)
        response = multi_callable(request)

        self.assertEqual(list(response), list(expected_response))
        self.assertTrue(self._client_interceptor.intercepted)
        self.assertTrue(self._server_interceptor.intercepted)

    def testStreamUnaryInterception(self):
        multi_callable = self._service.stream_unary_multi_callable
        requests = [b'\x01', b'\x02']
        expected_response = self._service.handler.handle_stream_unary(
            iter(requests), None)
        response = multi_callable(iter(requests))

        self.assertEqual(response, expected_response)
        self.assertTrue(self._client_interceptor.intercepted)
        self.assertTrue(self._server_interceptor.intercepted)

    def testStreamUnaryInterceptionWithCall(self):
        multi_callable = self._service.stream_unary_multi_callable
        requests = [b'\x01', b'\x02']
        expected_response = self._service.handler.handle_stream_unary(
            iter(requests), None)
        response, call = multi_callable.with_call(iter(requests))

        self.assertEqual(response, expected_response)
        self.assertIs(grpc.StatusCode.OK, call.code())
        self.assertTrue(self._client_interceptor.intercepted)
        self.assertTrue(self._server_interceptor.intercepted)

    def testStreamUnaryInterceptionFuture(self):
        multi_callable = self._service.stream_unary_multi_callable
        requests = [b'\x01', b'\x02']
        expected_response = self._service.handler.handle_stream_unary(
            iter(requests), None)
        response = multi_callable.future(iter(requests)).result()

        self.assertEqual(response, expected_response)
        self.assertTrue(self._client_interceptor.intercepted)
        self.assertTrue(self._server_interceptor.intercepted)

    def testStreamStreamInterception(self):
        multi_callable = self._service.stream_stream_multi_callable
        requests = [b'\x01', b'\x02']
        expected_response = self._service.handler.handle_stream_stream(
            iter(requests), None)
        response = multi_callable(iter(requests))

        self.assertEqual(list(response), list(expected_response))
        self.assertTrue(self._client_interceptor.intercepted)
        self.assertTrue(self._server_interceptor.intercepted)


class MultiInterceptorTest(unittest.TestCase):
    """Test that you can chain multiple interceptors together."""

    def setUp(self):
        self._client_interceptors = [ClientInterceptor(), ClientInterceptor()]
        self._server_interceptors = [ServerInterceptor(), ServerInterceptor()]
        self._service = Service(self._client_interceptors,
                                self._server_interceptors)

    def _clear(self):
        for client_interceptor in self._client_interceptors:
            client_interceptor.intercepted = False

        for server_interceptor in self._server_interceptors:
            server_interceptor.intercepted = False

    def testUnaryUnaryMultiInterception(self):
        multi_callable = self._service.unary_unary_multi_callable
        request = b'\x01'
        expected_response = self._service.handler.handle_unary_unary(request,
                                                                     None)
        response = multi_callable(request)

        self.assertEqual(response, expected_response)
        for client_interceptor in self._client_interceptors:
            self.assertTrue(client_interceptor.intercepted)
        for server_interceptor in self._server_interceptors:
            self.assertTrue(server_interceptor.intercepted)
