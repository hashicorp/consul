package io.opentracing.contrib;

import static org.junit.Assert.assertEquals;

import org.junit.Test;

import io.grpc.Context;
import io.opentracing.Span;
import io.opentracing.Tracer;
import io.opentracing.mock.MockTracer;

public class ActiveSpanSourceTest {
	
	Tracer tracer = new MockTracer();
	
	@Test
	public void TestDefaultNone() {
		ActiveSpanSource ss = ActiveSpanSource.NONE;
		assertEquals("active span should always be null", ss.getActiveSpan(), null);
		
		Span span = tracer.buildSpan("s0").start();
		Context ctx = Context.current().withValue(OpenTracingContextKey.getKey(), span);
		Context previousCtx = ctx.attach();

		assertEquals("active span should always be null", ss.getActiveSpan(), null);

		ctx.detach(previousCtx);
		span.finish();
	}
	
	@Test 
	public void TestDefaultGrpc() {
		ActiveSpanSource ss = ActiveSpanSource.GRPC_CONTEXT;
		assertEquals("active span should be null, no span in OpenTracingContextKey", ss.getActiveSpan(), null);
		
		Span span = tracer.buildSpan("s0").start();
		Context ctx = Context.current().withValue(OpenTracingContextKey.getKey(), span);
		Context previousCtx = ctx.attach();

		assertEquals("active span should be OpenTracingContextKey.activeSpan()", ss.getActiveSpan(), span);

		ctx.detach(previousCtx);
		span.finish();
		
		assertEquals("active span should be null, no span in OpenTracingContextKey", ss.getActiveSpan(), null);
	}
	
}