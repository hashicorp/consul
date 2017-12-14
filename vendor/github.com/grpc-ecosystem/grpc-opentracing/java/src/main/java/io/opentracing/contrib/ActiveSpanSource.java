package io.opentracing.contrib;

import io.opentracing.Span;
import io.opentracing.contrib.OpenTracingContextKey;

/**
 * An interface that defines how to get the current active span
 */
public interface ActiveSpanSource {

    /**
     * ActiveSpanSource implementation that always returns
     *  null as the active span
     */
    public static ActiveSpanSource NONE = new ActiveSpanSource() {
        @Override
        public Span getActiveSpan() {
            return null;
        }
    };
    
    /**
     * ActiveSpanSource implementation that returns the
     *  current span stored in the GRPC context under
     *  {@link OpenTracingContextKey}
     */
    public static ActiveSpanSource GRPC_CONTEXT = new ActiveSpanSource() {
        @Override 
        public Span getActiveSpan() {
            return OpenTracingContextKey.activeSpan();
        }
    };

    /**
     * @return the active span
     */
    public Span getActiveSpan();
}