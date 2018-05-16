import abc

import six


class UnaryClientInfo(six.with_metaclass(abc.ABCMeta)):
    """Consists of various information about a unary RPC on the invocation-side.

  Attributes:
    full_method: A string of the full RPC method, i.e., /package.service/method.
    timeout: The length of time in seconds to wait for the computation to
      terminate or be cancelled, or None if this method should block until
      the computation is terminated or is cancelled no matter how long that
      takes.
  """


class StreamClientInfo(six.with_metaclass(abc.ABCMeta)):
    """Consists of various information about a stream RPC on the invocation-side.

  Attributes:
    full_method: A string of the full RPC method, i.e., /package.service/method.
    is_client_stream: Indicates whether the RPC is client-streaming.
    is_server_stream: Indicates whether the RPC is server-streaming.
    timeout: The length of time in seconds to wait for the computation to
      terminate or be cancelled, or None if this method should block until
      the computation is terminated or is cancelled no matter how long that
      takes.
  """


class UnaryClientInterceptor(six.with_metaclass(abc.ABCMeta)):
    """Affords intercepting unary-unary RPCs on the invocation-side."""

    @abc.abstractmethod
    def intercept_unary(self, request, metadata, client_info, invoker):
        """Intercepts unary-unary RPCs on the invocation-side.

    Args:
      request: The request value for the RPC.
      metadata: Optional :term:`metadata` to be transmitted to the
        service-side of the RPC.
      client_info: A UnaryClientInfo containing various information about
        the RPC.
      invoker: The handler to complete the RPC on the client. It is the
        interceptor's responsibility to call it.

    Returns:
      The result from calling invoker(request, metadata).
    """
        raise NotImplementedError()


class StreamClientInterceptor(six.with_metaclass(abc.ABCMeta)):
    """Affords intercepting stream RPCs on the invocation-side."""

    @abc.abstractmethod
    def intercept_stream(self, request_or_iterator, metadata, client_info,
                         invoker):
        """Intercepts stream RPCs on the invocation-side.

    Args:
      request_or_iterator: The request value for the RPC if
        `client_info.is_client_stream` is `false`; otherwise, an iterator of
        request values.
      metadata: Optional :term:`metadata` to be transmitted to the service-side
        of the RPC.
      client_info: A StreamClientInfo containing various information about
        the RPC.
      invoker:  The handler to complete the RPC on the client. It is the
        interceptor's responsibility to call it.

      Returns:
        The result from calling invoker(metadata).
    """
        raise NotImplementedError()


def intercept_channel(channel, *interceptors):
    """Creates an intercepted channel.

  Args:
    channel: A Channel.
    interceptors: Zero or more UnaryClientInterceptors or
      StreamClientInterceptors

  Returns:
    A Channel.

  Raises:
    TypeError: If an interceptor derives from neither UnaryClientInterceptor
      nor StreamClientInterceptor.
  """
    from grpc_opentracing.grpcext import _interceptor
    return _interceptor.intercept_channel(channel, *interceptors)


class UnaryServerInfo(six.with_metaclass(abc.ABCMeta)):
    """Consists of various information about a unary RPC on the service-side.

  Attributes:
    full_method: A string of the full RPC method, i.e., /package.service/method.
  """


class StreamServerInfo(six.with_metaclass(abc.ABCMeta)):
    """Consists of various information about a stream RPC on the service-side.

  Attributes:
    full_method: A string of the full RPC method, i.e., /package.service/method.
    is_client_stream: Indicates whether the RPC is client-streaming.
    is_server_stream: Indicates whether the RPC is server-streaming.
  """


class UnaryServerInterceptor(six.with_metaclass(abc.ABCMeta)):
    """Affords intercepting unary-unary RPCs on the service-side."""

    @abc.abstractmethod
    def intercept_unary(self, request, servicer_context, server_info, handler):
        """Intercepts unary-unary RPCs on the service-side.

    Args:
      request: The request value for the RPC.
      servicer_context: A ServicerContext.
      server_info: A UnaryServerInfo containing various information about
        the RPC.
      handler:  The handler to complete the RPC on the server. It is the
        interceptor's responsibility to call it.

    Returns:
      The result from calling handler(request, servicer_context).
    """
        raise NotImplementedError()


class StreamServerInterceptor(six.with_metaclass(abc.ABCMeta)):
    """Affords intercepting stream RPCs on the service-side."""

    @abc.abstractmethod
    def intercept_stream(self, request_or_iterator, servicer_context,
                         server_info, handler):
        """Intercepts stream RPCs on the service-side.

    Args:
      request_or_iterator: The request value for the RPC if
        `server_info.is_client_stream` is `False`; otherwise, an iterator of
        request values.
      servicer_context: A ServicerContext.
      server_info: A StreamServerInfo containing various information about
        the RPC.
      handler:  The handler to complete the RPC on the server. It is the
        interceptor's responsibility to call it.

      Returns:
        The result from calling handler(servicer_context).
    """
        raise NotImplementedError()


def intercept_server(server, *interceptors):
    """Creates an intercepted server.

  Args:
    server: A Server.
    interceptors: Zero or more UnaryServerInterceptors or
      StreamServerInterceptors

  Returns:
    A Server.

  Raises:
    TypeError: If an interceptor derives from neither UnaryServerInterceptor
      nor StreamServerInterceptor.
  """
    from grpc_opentracing.grpcext import _interceptor
    return _interceptor.intercept_server(server, *interceptors)


###################################  __all__  #################################

__all__ = ('UnaryClientInterceptor', 'StreamClientInfo',
           'StreamClientInterceptor', 'UnaryServerInfo', 'StreamServerInfo',
           'UnaryServerInterceptor', 'StreamServerInterceptor',
           'intercept_channel', 'intercept_server',)
