package io.opentracing.contrib;

import static org.junit.Assert.assertEquals;

import org.junit.Test;

import io.grpc.Context;
import io.opentracing.Span;
import io.opentracing.Tracer;
import io.opentracing.mock.MockTracer;

public class OpenTracingContextKeyTest {

	Tracer tracer = new MockTracer();

	@Test
	public void TestGetKey() {
		Context.Key<Span> key = OpenTracingContextKey.getKey();
		assertEquals("Key should have correct name", key.toString(), (OpenTracingContextKey.KEY_NAME));
	}

	@Test
	public void TestNoActiveSpan() {
		assertEquals("activeSpan() should return null when no span is active", 
				OpenTracingContextKey.activeSpan(), null);
	}

	@Test
	public void TestGetActiveSpan() {
		Span span = tracer.buildSpan("s0").start();
		Context ctx = Context.current().withValue(OpenTracingContextKey.getKey(), span);
		Context previousCtx = ctx.attach();

		assertEquals(OpenTracingContextKey.activeSpan(), span);

		ctx.detach(previousCtx);
		span.finish();

		assertEquals(OpenTracingContextKey.activeSpan(), null);
	}

	@Test
	public void TestMultipleContextLayers() {
		Span parentSpan = tracer.buildSpan("s0").start();
		Context parentCtx = Context.current().withValue(OpenTracingContextKey.getKey(), parentSpan);
		Context previousCtx = parentCtx.attach();

		Span childSpan = tracer.buildSpan("s1").start();
		Context childCtx = Context.current().withValue(OpenTracingContextKey.getKey(), childSpan);
		parentCtx = childCtx.attach();

		assertEquals(OpenTracingContextKey.activeSpan(), childSpan);

		childCtx.detach(parentCtx);
		childSpan.finish();

		assertEquals(OpenTracingContextKey.activeSpan(), parentSpan);

		parentCtx.detach(previousCtx);
		parentSpan.finish();

		assertEquals(OpenTracingContextKey.activeSpan(), null);
	}

	@Test
	public void TestWrappedCall() {
		
	}
}