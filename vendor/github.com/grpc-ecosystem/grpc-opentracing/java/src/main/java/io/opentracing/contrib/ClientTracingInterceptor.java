package io.opentracing.contrib;

import com.google.common.collect.ImmutableMap;
import io.grpc.*;
import io.opentracing.Span;
import io.opentracing.Tracer;
import io.opentracing.propagation.Format;
import io.opentracing.propagation.TextMap;

import javax.annotation.Nullable;
import java.util.Map.Entry;
import java.util.*;
import java.util.concurrent.TimeUnit;

/** 
 * An intercepter that applies tracing via OpenTracing to all client requests. 
 */ 
public class ClientTracingInterceptor implements ClientInterceptor {
    
    private final Tracer tracer;
    private final OperationNameConstructor operationNameConstructor;
    private final boolean streaming;
    private final boolean verbose;
    private final Set<ClientRequestAttribute> tracedAttributes;
    private final ActiveSpanSource activeSpanSource;

    /**
     * @param tracer to use to trace requests
     */
    public ClientTracingInterceptor(Tracer tracer) {
        this.tracer = tracer;
        this.operationNameConstructor = OperationNameConstructor.DEFAULT;
        this.streaming = false;
        this.verbose = false;
        this.tracedAttributes = new HashSet<ClientRequestAttribute>();
        this.activeSpanSource = ActiveSpanSource.GRPC_CONTEXT;
    }

    private ClientTracingInterceptor(Tracer tracer, OperationNameConstructor operationNameConstructor, boolean streaming,
        boolean verbose, Set<ClientRequestAttribute> tracedAttributes, ActiveSpanSource activeSpanSource) {
        this.tracer = tracer;
        this.operationNameConstructor = operationNameConstructor;
        this.streaming = streaming;
        this.verbose = verbose;
        this.tracedAttributes = tracedAttributes;
        this.activeSpanSource = activeSpanSource;
    }

    /**
     * Use this intercepter to trace all requests made by this client channel.
     * @param channel to be traced
     * @return intercepted channel
     */ 
    public Channel intercept(Channel channel) {
        return ClientInterceptors.intercept(channel, this);
    }

    @Override
    public <ReqT, RespT> ClientCall<ReqT, RespT> interceptCall(
        MethodDescriptor<ReqT, RespT> method, 
        CallOptions callOptions, 
        Channel next
    ) {
        final String operationName = operationNameConstructor.constructOperationName(method);

        Span activeSpan = this.activeSpanSource.getActiveSpan();
        final Span span = createSpanFromParent(activeSpan, operationName);

        for (ClientRequestAttribute attr : this.tracedAttributes) {
            switch (attr) {
                case ALL_CALL_OPTIONS:
                    span.setTag("grpc.call_options", callOptions.toString());
                    break;
                case AUTHORITY:
                    if (callOptions.getAuthority() == null) {
                        span.setTag("grpc.authority", "null");
                    } else {
                        span.setTag("grpc.authority", callOptions.getAuthority());                        
                    }
                    break;
                case COMPRESSOR:
                    if (callOptions.getCompressor() == null) {
                        span.setTag("grpc.compressor", "null");
                    } else {
                        span.setTag("grpc.compressor", callOptions.getCompressor());
                    }
                    break;
                case DEADLINE:
                    if (callOptions.getDeadline() == null) {
                        span.setTag("grpc.deadline_millis", "null");
                    } else {
                        span.setTag("grpc.deadline_millis", callOptions.getDeadline().timeRemaining(TimeUnit.MILLISECONDS));
                    }
                    break;
                case METHOD_NAME:
                    span.setTag("grpc.method_name", method.getFullMethodName());
                    break;
                case METHOD_TYPE:
                    if (method.getType() == null) {
                        span.setTag("grpc.method_type", "null");
                    } else {
                        span.setTag("grpc.method_type", method.getType().toString());
                    }
                    break;
                case HEADERS:
                	break;
            }
        }

        return new ForwardingClientCall.SimpleForwardingClientCall<ReqT, RespT>(next.newCall(method, callOptions)) {

            @Override
            public void start(Listener<RespT> responseListener, Metadata headers) {
                if (verbose) {
                    span.log("Started call");
                }
                if (tracedAttributes.contains(ClientRequestAttribute.HEADERS)) {
                    span.setTag("grpc.headers", headers.toString()); 
                }

                tracer.inject(span.context(), Format.Builtin.HTTP_HEADERS, new TextMap() {
                    @Override
                    public void put(String key, String value) {
                        Metadata.Key<String> headerKey = Metadata.Key.of(key, Metadata.ASCII_STRING_MARSHALLER);
                        headers.put(headerKey, value);
                    }
					@Override
					public Iterator<Entry<String, String>> iterator() {
						throw new UnsupportedOperationException(
								"TextMapInjectAdapter should only be used with Tracer.inject()");
					}
                });

                Listener<RespT> tracingResponseListener = new ForwardingClientCallListener
                    .SimpleForwardingClientCallListener<RespT>(responseListener) {

                    @Override
                    public void onHeaders(Metadata headers) {
                        if (verbose) { span.log(ImmutableMap.of("Response headers received", headers.toString())); }
                        delegate().onHeaders(headers);
                    }

                    @Override
                    public void onMessage(RespT message) {
                        if (streaming || verbose) { span.log("Response received"); }
                        delegate().onMessage(message);
                    }

                    @Override 
                    public void onClose(Status status, Metadata trailers) {
                        if (verbose) { 
                            if (status.getCode().value() == 0) { span.log("Call closed"); }
                            else { span.log(ImmutableMap.of("Call failed", status.getDescription())); }
                        }
                        span.finish();
                        delegate().onClose(status, trailers);
                    }
                };
                delegate().start(tracingResponseListener, headers);
            }

            @Override 
            public void cancel(@Nullable String message, @Nullable Throwable cause) {
                String errorMessage;
                if (message == null) {
                    errorMessage = "Error";
                } else {
                    errorMessage = message;
                }
                if (cause == null) {
                    span.log(errorMessage);
                } else {
                    span.log(ImmutableMap.of(errorMessage, cause.getMessage()));
                }
                delegate().cancel(message, cause);
            }

            @Override
            public void halfClose() {
                if (streaming) { span.log("Finished sending messages"); }
                delegate().halfClose();
            }

            @Override
            public void sendMessage(ReqT message) {
                if (streaming || verbose) { span.log("Message sent"); }
                delegate().sendMessage(message);
            }
        };
    }
    
    private Span createSpanFromParent(Span parentSpan, String operationName) {
        if (parentSpan == null) {
            return tracer.buildSpan(operationName).startManual();
        } else {
            return tracer.buildSpan(operationName).asChildOf(parentSpan).startManual();
        }
    }

    /**
     * Builds the configuration of a ClientTracingInterceptor.
     */
    public static class Builder {

        private Tracer tracer;
        private OperationNameConstructor operationNameConstructor;
        private boolean streaming;
        private boolean verbose;
        private Set<ClientRequestAttribute> tracedAttributes;
        private ActiveSpanSource activeSpanSource;  

        /**
         * @param tracer to use for this intercepter
         * Creates a Builder with default configuration
         */
        public Builder(Tracer tracer) {
            this.tracer = tracer;
            this.operationNameConstructor = OperationNameConstructor.DEFAULT;
            this.streaming = false;
            this.verbose = false;
            this.tracedAttributes = new HashSet<ClientRequestAttribute>();
            this.activeSpanSource = ActiveSpanSource.GRPC_CONTEXT;
        } 

        /**
         * @param operationNameConstructor to name all spans created by this intercepter
         * @return this Builder with configured operation name
         */
        public Builder withOperationName(OperationNameConstructor operationNameConstructor) {
            this.operationNameConstructor = operationNameConstructor;
            return this;
        } 

        /**
         * Logs streaming events to client spans.
         * @return this Builder configured to log streaming events
         */
        public Builder withStreaming() {
            this.streaming = true;
            return this;
        }

        /**
         * @param tracedAttributes to set as tags on client spans
         *  created by this intercepter
         * @return this Builder configured to trace attributes
         */
        public Builder withTracedAttributes(ClientRequestAttribute... tracedAttributes) {
            this.tracedAttributes = new HashSet<ClientRequestAttribute>(
                Arrays.asList(tracedAttributes));
            return this;
        }

        /**
         * Logs all request life-cycle events to client spans.
         * @return this Builder configured to be verbose
         */
        public Builder withVerbosity() {
            this.verbose = true;
            return this;
        }

        /**
         * @param activeSpanSource that provides a method of getting the 
         *  active span before the client call
         * @return this Builder configured to start client span as children 
         *  of the span returned by activeSpanSource.getActiveSpan()
         */
        public Builder withActiveSpanSource(ActiveSpanSource activeSpanSource) {
            this.activeSpanSource = activeSpanSource;
            return this;
        }

        /**
         * @return a ClientTracingInterceptor with this Builder's configuration
         */
        public ClientTracingInterceptor build() {
            return new ClientTracingInterceptor(this.tracer, this.operationNameConstructor, 
                this.streaming, this.verbose, this.tracedAttributes, this.activeSpanSource);
        }
    }

    public enum ClientRequestAttribute {
        METHOD_TYPE,
        METHOD_NAME,
        DEADLINE,
        COMPRESSOR,
        AUTHORITY,
        ALL_CALL_OPTIONS,
        HEADERS
    }
}
