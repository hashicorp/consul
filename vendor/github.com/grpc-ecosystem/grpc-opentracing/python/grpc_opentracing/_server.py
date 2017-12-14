"""Implementation of the service-side open-tracing interceptor."""

import sys
import logging
import re

import grpc
from grpc_opentracing import grpcext, ActiveSpanSource
from grpc_opentracing._utilities import get_method_type, get_deadline_millis,\
    log_or_wrap_request_or_iterator, RpcInfo
import opentracing
from opentracing.ext import tags as ot_tags


class _OpenTracingServicerContext(grpc.ServicerContext, ActiveSpanSource):

    def __init__(self, servicer_context, active_span):
        self._servicer_context = servicer_context
        self._active_span = active_span
        self.code = grpc.StatusCode.OK
        self.details = None

    def is_active(self, *args, **kwargs):
        return self._servicer_context.is_active(*args, **kwargs)

    def time_remaining(self, *args, **kwargs):
        return self._servicer_context.time_remaining(*args, **kwargs)

    def cancel(self, *args, **kwargs):
        return self._servicer_context.cancel(*args, **kwargs)

    def add_callback(self, *args, **kwargs):
        return self._servicer_context.add_callback(*args, **kwargs)

    def invocation_metadata(self, *args, **kwargs):
        return self._servicer_context.invocation_metadata(*args, **kwargs)

    def peer(self, *args, **kwargs):
        return self._servicer_context.peer(*args, **kwargs)

    def peer_identities(self, *args, **kwargs):
        return self._servicer_context.peer_identities(*args, **kwargs)

    def peer_identity_key(self, *args, **kwargs):
        return self._servicer_context.peer_identity_key(*args, **kwargs)

    def auth_context(self, *args, **kwargs):
        return self._servicer_context.auth_context(*args, **kwargs)

    def send_initial_metadata(self, *args, **kwargs):
        return self._servicer_context.send_initial_metadata(*args, **kwargs)

    def set_trailing_metadata(self, *args, **kwargs):
        return self._servicer_context.set_trailing_metadata(*args, **kwargs)

    def set_code(self, code):
        self.code = code
        return self._servicer_context.set_code(code)

    def set_details(self, details):
        self.details = details
        return self._servicer_context.set_details(details)

    def get_active_span(self):
        return self._active_span


def _add_peer_tags(peer_str, tags):
    ipv4_re = r"ipv4:(?P<address>.+):(?P<port>\d+)"
    match = re.match(ipv4_re, peer_str)
    if match:
        tags[ot_tags.PEER_HOST_IPV4] = match.group('address')
        tags[ot_tags.PEER_PORT] = match.group('port')
        return
    ipv6_re = r"ipv6:\[(?P<address>.+)\]:(?P<port>\d+)"
    match = re.match(ipv6_re, peer_str)
    if match:
        tags[ot_tags.PEER_HOST_IPV6] = match.group('address')
        tags[ot_tags.PEER_PORT] = match.group('port')
        return
    logging.warning('Unrecognized peer: \"%s\"', peer_str)


# On the service-side, errors can be signaled either by exceptions or by calling
# `set_code` on the `servicer_context`. This function checks for the latter and
# updates the span accordingly.
def _check_error_code(span, servicer_context, rpc_info):
    if servicer_context.code != grpc.StatusCode.OK:
        span.set_tag('error', True)
        error_log = {'event': 'error', 'error.kind': str(servicer_context.code)}
        if servicer_context.details is not None:
            error_log['message'] = servicer_context.details
        span.log_kv(error_log)
        rpc_info.error = servicer_context.code


class OpenTracingServerInterceptor(grpcext.UnaryServerInterceptor,
                                   grpcext.StreamServerInterceptor):

    def __init__(self, tracer, log_payloads, span_decorator):
        self._tracer = tracer
        self._log_payloads = log_payloads
        self._span_decorator = span_decorator

    def _start_span(self, servicer_context, method):
        span_context = None
        error = None
        metadata = servicer_context.invocation_metadata()
        try:
            if metadata:
                span_context = self._tracer.extract(
                    opentracing.Format.HTTP_HEADERS, dict(metadata))
        except (opentracing.UnsupportedFormatException,
                opentracing.InvalidCarrierException,
                opentracing.SpanContextCorruptedException) as e:
            logging.exception('tracer.extract() failed')
            error = e
        tags = {
            ot_tags.COMPONENT: 'grpc',
            ot_tags.SPAN_KIND: ot_tags.SPAN_KIND_RPC_SERVER
        }
        _add_peer_tags(servicer_context.peer(), tags)
        span = self._tracer.start_span(
            operation_name=method, child_of=span_context, tags=tags)
        if error is not None:
            span.log_kv({'event': 'error', 'error.object': error})
        return span

    def intercept_unary(self, request, servicer_context, server_info, handler):
        with self._start_span(servicer_context,
                              server_info.full_method) as span:
            rpc_info = RpcInfo(
                full_method=server_info.full_method,
                metadata=servicer_context.invocation_metadata(),
                timeout=servicer_context.time_remaining(),
                request=request)
            if self._log_payloads:
                span.log_kv({'request': request})
            servicer_context = _OpenTracingServicerContext(
                servicer_context, span)
            try:
                response = handler(request, servicer_context)
            except:
                e = sys.exc_info()[0]
                span.set_tag('error', True)
                span.log_kv({'event': 'error', 'error.object': e})
                rpc_info.error = e
                if self._span_decorator is not None:
                    self._span_decorator(span, rpc_info)
                raise
            if self._log_payloads:
                span.log_kv({'response': response})
            _check_error_code(span, servicer_context, rpc_info)
            rpc_info.response = response
            if self._span_decorator is not None:
                self._span_decorator(span, rpc_info)
            return response

    # For RPCs that stream responses, the result can be a generator. To record
    # the span across the generated responses and detect any errors, we wrap the
    # result in a new generator that yields the response values.
    def _intercept_server_stream(self, request_or_iterator, servicer_context,
                                 server_info, handler):
        with self._start_span(servicer_context,
                              server_info.full_method) as span:
            rpc_info = RpcInfo(
                full_method=server_info.full_method,
                metadata=servicer_context.invocation_metadata(),
                timeout=servicer_context.time_remaining())
            if not server_info.is_client_stream:
                rpc_info.request = request_or_iterator
            if self._log_payloads:
                request_or_iterator = log_or_wrap_request_or_iterator(
                    span, server_info.is_client_stream, request_or_iterator)
            servicer_context = _OpenTracingServicerContext(
                servicer_context, span)
            try:
                result = handler(request_or_iterator, servicer_context)
                for response in result:
                    if self._log_payloads:
                        span.log_kv({'response': response})
                    yield response
            except:
                e = sys.exc_info()[0]
                span.set_tag('error', True)
                span.log_kv({'event': 'error', 'error.object': e})
                rpc_info.error = e
                if self._span_decorator is not None:
                    self._span_decorator(span, rpc_info)
                raise
            _check_error_code(span, servicer_context, rpc_info)
            if self._span_decorator is not None:
                self._span_decorator(span, rpc_info)

    def intercept_stream(self, request_or_iterator, servicer_context,
                         server_info, handler):
        if server_info.is_server_stream:
            return self._intercept_server_stream(
                request_or_iterator, servicer_context, server_info, handler)
        with self._start_span(servicer_context,
                              server_info.full_method) as span:
            rpc_info = RpcInfo(
                full_method=server_info.full_method,
                metadata=servicer_context.invocation_metadata(),
                timeout=servicer_context.time_remaining())
            if self._log_payloads:
                request_or_iterator = log_or_wrap_request_or_iterator(
                    span, server_info.is_client_stream, request_or_iterator)
            servicer_context = _OpenTracingServicerContext(
                servicer_context, span)
            try:
                response = handler(request_or_iterator, servicer_context)
            except:
                e = sys.exc_info()[0]
                span.set_tag('error', True)
                span.log_kv({'event': 'error', 'error.object': e})
                rpc_info.error = e
                if self._span_decorator is not None:
                    self._span_decorator(span, rpc_info)
                raise
            if self._log_payloads:
                span.log_kv({'response': response})
            _check_error_code(span, servicer_context, rpc_info)
            rpc_info.response = response
            if self._span_decorator is not None:
                self._span_decorator(span, rpc_info)
            return response
