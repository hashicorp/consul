package io.opentracing.contrib;

import io.grpc.ManagedChannel;
import io.grpc.ManagedChannelBuilder;

public class TracedClient {
	private final ManagedChannel channel;
	private final GreeterGrpc.GreeterBlockingStub blockingStub;
	
	public TracedClient(String host, int port, ClientTracingInterceptor tracingInterceptor) {
		channel = ManagedChannelBuilder.forAddress(host, port)
				.usePlaintext(true)
				.build();
		
		if(tracingInterceptor==null) {
			blockingStub = GreeterGrpc.newBlockingStub(channel);
		} else {
			blockingStub = GreeterGrpc.newBlockingStub(tracingInterceptor.intercept(channel));
		}		
	}
	
	void shutdown() throws InterruptedException {
		channel.shutdown();
	}
	
	boolean greet(String name) {
		HelloRequest request = HelloRequest.newBuilder().setName(name).build();
		try {
			blockingStub.sayHello(request);
		} catch (Exception e) {
			return false;
		} finally {
			try { this.shutdown(); }
			catch (Exception e) { return false; }
		}
		return true;
		
	}
}
