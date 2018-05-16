package io.opentracing.contrib;

import java.io.IOException;

import io.grpc.Server;
import io.grpc.ServerBuilder;
import io.grpc.stub.StreamObserver;

public class TracedService {
	private int port = 50051;
	private Server server;
	
	void start() throws IOException {
		server = ServerBuilder.forPort(port)
				.addService(new GreeterImpl())
				.build()
				.start();
		
		Runtime.getRuntime().addShutdownHook(new Thread() {
			@Override
			public void run() {
				TracedService.this.stop();
			}
		});
	}
	
	void startWithInterceptor(ServerTracingInterceptor tracingInterceptor) throws IOException {
				
		server = ServerBuilder.forPort(port)
				.addService(tracingInterceptor.intercept(new GreeterImpl()))
				.build()
				.start();
		
		Runtime.getRuntime().addShutdownHook(new Thread() {
			@Override
			public void run() {
				TracedService.this.stop();
			}
		});
	}
	
	void blockUntilShutdown() throws InterruptedException {
		if (server != null) {
			server.awaitTermination();
		}
	}
	
	void stop() {
		if (server != null) {
			server.shutdown();
		}
	}
	
	private class GreeterImpl extends GreeterGrpc.GreeterImplBase {
		@Override
		public void sayHello(HelloRequest req, StreamObserver<HelloReply> responseObserver) {
			HelloReply reply = HelloReply.newBuilder().setMessage("Hello").build();
			responseObserver.onNext(reply);
			responseObserver.onCompleted();
			
		}
	}
}
