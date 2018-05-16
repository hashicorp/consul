"""Implementation of the invocation-side open-tracing interceptor."""

import sys
import logging
import time

from six import iteritems

import grpc
from grpc_opentracing import grpcext
from grpc_opentracing._utilities import get_method_type, get_deadline_millis,\
    log_or_wrap_request_or_iterator, RpcInfo
import opentracing
from opentracing.ext import tags as ot_tags


class _GuardedSpan(object):

    def __init__(self, span):
        self.span = span
        self._engaged = True

    def __enter__(self):
        self.span.__enter__()
        return self

    def __exit__(self, *args, **kwargs):
        if self._engaged:
            return self.span.__exit__(*args, **kwargs)
        else:
            return False

    def release(self):
        self._engaged = False
        return self.span


def _inject_span_context(tracer, span, metadata):
    headers = {}
    try:
        tracer.inject(span.context, opentracing.Format.HTTP_HEADERS, headers)
    except (opentracing.UnsupportedFormatException,
            opentracing.InvalidCarrierException,
            opentracing.SpanContextCorruptedException) as e:
        logging.exception('tracer.inject() failed')
        span.log_kv({'event': 'error', 'error.object': e})
        return metadata
    metadata = () if metadata is None else tuple(metadata)
    return metadata + tuple(iteritems(headers))


def _make_future_done_callback(span, rpc_info, log_payloads, span_decorator):

    def callback(response_future):
        with span:
            code = response_future.code()
            if code != grpc.StatusCode.OK:
                span.set_tag('error', True)
                error_log = {'event': 'error', 'error.kind': str(code)}
                details = response_future.details()
                if details is not None:
                    error_log['message'] = details
                span.log_kv(error_log)
                rpc_info.error = code
                if span_decorator is not None:
                    span_decorator(span, rpc_info)
                return
            response = response_future.result()
            rpc_info.response = response
            if log_payloads:
                span.log_kv({'response': response})
            if span_decorator is not None:
                span_decorator(span, rpc_info)

    return callback


class OpenTracingClientInterceptor(grpcext.UnaryClientInterceptor,
                                   grpcext.StreamClientInterceptor):

    def __init__(self, tracer, active_span_source, log_payloads,
                 span_decorator):
        self._tracer = tracer
        self._active_span_source = active_span_source
        self._log_payloads = log_payloads
        self._span_decorator = span_decorator

    def _start_span(self, method):
        active_span_context = None
        if self._active_span_source is not None:
            active_span = self._active_span_source.get_active_span()
            if active_span is not None:
                active_span_context = active_span.context
        tags = {
            ot_tags.COMPONENT: 'grpc',
            ot_tags.SPAN_KIND: ot_tags.SPAN_KIND_RPC_CLIENT
        }
        return self._tracer.start_span(
            operation_name=method, child_of=active_span_context, tags=tags)

    def _trace_result(self, guarded_span, rpc_info, result):
        # If the RPC is called asynchronously, release the guard and add a callback
        # so that the span can be finished once the future is done.
        if isinstance(result, grpc.Future):
            result.add_done_callback(
                _make_future_done_callback(guarded_span.release(
                ), rpc_info, self._log_payloads, self._span_decorator))
            return result
        response = result
        # Handle the case when the RPC is initiated via the with_call
        # method and the result is a tuple with the first element as the
        # response.
        # http://www.grpc.io/grpc/python/grpc.html#grpc.UnaryUnaryMultiCallable.with_call
        if isinstance(result, tuple):
            response = result[0]
        rpc_info.response = response
        if self._log_payloads:
            guarded_span.span.log_kv({'response': response})
        if self._span_decorator is not None:
            self._span_decorator(guarded_span.span, rpc_info)
        return result

    def _start_guarded_span(self, *args, **kwargs):
        return _GuardedSpan(self._start_span(*args, **kwargs))

    def intercept_unary(self, request, metadata, client_info, invoker):
        with self._start_guarded_span(client_info.full_method) as guarded_span:
            metadata = _inject_span_context(self._tracer, guarded_span.span,
                                            metadata)
            rpc_info = RpcInfo(
                full_method=client_info.full_method,
                metadata=metadata,
                timeout=client_info.timeout,
                request=request)
            if self._log_payloads:
                guarded_span.span.log_kv({'request': request})
            try:
                result = invoker(request, metadata)
            except:
                e = sys.exc_info()[0]
                guarded_span.span.set_tag('error', True)
                guarded_span.span.log_kv({'event': 'error', 'error.object': e})
                rpc_info.error = e
                if self._span_decorator is not None:
                    self._span_decorator(guarded_span.span, rpc_info)
                raise
            return self._trace_result(guarded_span, rpc_info, result)

    # For RPCs that stream responses, the result can be a generator. To record
    # the span across the generated responses and detect any errors, we wrap the
    # result in a new generator that yields the response values.
    def _intercept_server_stream(self, request_or_iterator, metadata,
                                 client_info, invoker):
        with self._start_span(client_info.full_method) as span:
            metadata = _inject_span_context(self._tracer, span, metadata)
            rpc_info = RpcInfo(
                full_method=client_info.full_method,
                metadata=metadata,
                timeout=client_info.timeout)
            if client_info.is_client_stream:
                rpc_info.request = request_or_iterator
            if self._log_payloads:
                request_or_iterator = log_or_wrap_request_or_iterator(
                    span, client_info.is_client_stream, request_or_iterator)
            try:
                result = invoker(request_or_iterator, metadata)
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
            if self._span_decorator is not None:
                self._span_decorator(span, rpc_info)

    def intercept_stream(self, request_or_iterator, metadata, client_info,
                         invoker):
        if client_info.is_server_stream:
            return self._intercept_server_stream(request_or_iterator, metadata,
                                                 client_info, invoker)
        with self._start_guarded_span(client_info.full_method) as guarded_span:
            metadata = _inject_span_context(self._tracer, guarded_span.span,
                                            metadata)
            rpc_info = RpcInfo(
                full_method=client_info.full_method,
                metadata=metadata,
                timeout=client_info.timeout,
                request=request_or_iterator)
            if self._log_payloads:
                request_or_iterator = log_or_wrap_request_or_iterator(
                    guarded_span.span, client_info.is_client_stream,
                    request_or_iterator)
            try:
                result = invoker(request_or_iterator, metadata)
            except:
                e = sys.exc_info()[0]
                guarded_span.span.set_tag('error', True)
                guarded_span.span.log_kv({'event': 'error', 'error.object': e})
                rpc_info.error = e
                if self._span_decorator is not None:
                    self._span_decorator(guarded_span.span, rpc_info)
                raise
            return self._trace_result(guarded_span, rpc_info, result)
