# Copyright 2012 Twitter Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
namespace java com.twitter.zipkin.thriftjava
#@namespace scala com.twitter.zipkin.thriftscala
namespace rb Zipkin

#************** Annotation.value **************
/**
 * The client sent ("cs") a request to a server. There is only one send per
 * span. For example, if there's a transport error, each attempt can be logged
 * as a WIRE_SEND annotation.
 *
 * If chunking is involved, each chunk could be logged as a separate
 * CLIENT_SEND_FRAGMENT in the same span.
 *
 * Annotation.host is not the server. It is the host which logged the send
 * event, almost always the client. When logging CLIENT_SEND, instrumentation
 * should also log the SERVER_ADDR.
 */
const string CLIENT_SEND = "cs"
/**
 * The client received ("cr") a response from a server. There is only one
 * receive per span. For example, if duplicate responses were received, each
 * can be logged as a WIRE_RECV annotation.
 *
 * If chunking is involved, each chunk could be logged as a separate
 * CLIENT_RECV_FRAGMENT in the same span.
 *
 * Annotation.host is not the server. It is the host which logged the receive
 * event, almost always the client. The actual endpoint of the server is
 * recorded separately as SERVER_ADDR when CLIENT_SEND is logged.
 */
const string CLIENT_RECV = "cr"
/**
 * The server sent ("ss") a response to a client. There is only one response
 * per span. If there's a transport error, each attempt can be logged as a
 * WIRE_SEND annotation.
 *
 * Typically, a trace ends with a server send, so the last timestamp of a trace
 * is often the timestamp of the root span's server send.
 *
 * If chunking is involved, each chunk could be logged as a separate
 * SERVER_SEND_FRAGMENT in the same span.
 *
 * Annotation.host is not the client. It is the host which logged the send
 * event, almost always the server. The actual endpoint of the client is
 * recorded separately as CLIENT_ADDR when SERVER_RECV is logged.
 */
const string SERVER_SEND = "ss"
/**
 * The server received ("sr") a request from a client. There is only one
 * request per span.  For example, if duplicate responses were received, each
 * can be logged as a WIRE_RECV annotation.
 *
 * Typically, a trace starts with a server receive, so the first timestamp of a
 * trace is often the timestamp of the root span's server receive.
 *
 * If chunking is involved, each chunk could be logged as a separate
 * SERVER_RECV_FRAGMENT in the same span.
 *
 * Annotation.host is not the client. It is the host which logged the receive
 * event, almost always the server. When logging SERVER_RECV, instrumentation
 * should also log the CLIENT_ADDR.
 */
const string SERVER_RECV = "sr"
/**
 * Optionally logs an attempt to send a message on the wire. Multiple wire send
 * events could indicate network retries. A lag between client or server send
 * and wire send might indicate queuing or processing delay.
 */
const string WIRE_SEND = "ws"
/**
 * Optionally logs an attempt to receive a message from the wire. Multiple wire
 * receive events could indicate network retries. A lag between wire receive
 * and client or server receive might indicate queuing or processing delay.
 */
const string WIRE_RECV = "wr"
/**
 * Optionally logs progress of a (CLIENT_SEND, WIRE_SEND). For example, this
 * could be one chunk in a chunked request.
 */
const string CLIENT_SEND_FRAGMENT = "csf"
/**
 * Optionally logs progress of a (CLIENT_RECV, WIRE_RECV). For example, this
 * could be one chunk in a chunked response.
 */
const string CLIENT_RECV_FRAGMENT = "crf"
/**
 * Optionally logs progress of a (SERVER_SEND, WIRE_SEND). For example, this
 * could be one chunk in a chunked response.
 */
const string SERVER_SEND_FRAGMENT = "ssf"
/**
 * Optionally logs progress of a (SERVER_RECV, WIRE_RECV). For example, this
 * could be one chunk in a chunked request.
 */
const string SERVER_RECV_FRAGMENT = "srf"

#***** BinaryAnnotation.key ******
/**
 * The domain portion of the URL or host header. Ex. "mybucket.s3.amazonaws.com"
 *
 * Used to filter by host as opposed to ip address.
 */
const string HTTP_HOST = "http.host"

/**
 * The HTTP method, or verb, such as "GET" or "POST".
 *
 * Used to filter against an http route.
 */
const string HTTP_METHOD = "http.method"

/**
 * The absolute http path, without any query parameters. Ex. "/objects/abcd-ff"
 *
 * Used to filter against an http route, portably with zipkin v1.
 *
 * In zipkin v1, only equals filters are supported. Dropping query parameters makes the number
 * of distinct URIs less. For example, one can query for the same resource, regardless of signing
 * parameters encoded in the query line. This does not reduce cardinality to a HTTP single route.
 * For example, it is common to express a route as an http URI template like
 * "/resource/{resource_id}". In systems where only equals queries are available, searching for
 * http/path=/resource won't match if the actual request was /resource/abcd-ff.
 *
 * Historical note: This was commonly expressed as "http.uri" in zipkin, even though it was most
 * often just a path.
 */
const string HTTP_PATH = "http.path"

/**
 * The entire URL, including the scheme, host and query parameters if available. Ex.
 * "https://mybucket.s3.amazonaws.com/objects/abcd-ff?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Algorithm=AWS4-HMAC-SHA256..."
 *
 * Combined with HTTP_METHOD, you can understand the fully-qualified request line.
 *
 * This is optional as it may include private data or be of considerable length.
 */
const string HTTP_URL = "http.url"

/**
 * The HTTP status code, when not in 2xx range. Ex. "503"
 *
 * Used to filter for error status.
 */
const string HTTP_STATUS_CODE = "http.status_code"

/**
 * The size of the non-empty HTTP request body, in bytes. Ex. "16384"
 *
 * Large uploads can exceed limits or contribute directly to latency.
 */
const string HTTP_REQUEST_SIZE = "http.request.size"

/**
 * The size of the non-empty HTTP response body, in bytes. Ex. "16384"
 *
 * Large downloads can exceed limits or contribute directly to latency.
 */
const string HTTP_RESPONSE_SIZE = "http.response.size"

/**
 * The value of "lc" is the component or namespace of a local span.
 *
 * BinaryAnnotation.host adds service context needed to support queries.
 *
 * Local Component("lc") supports three key features: flagging, query by
 * service and filtering Span.name by namespace.
 *
 * While structurally the same, local spans are fundamentally different than
 * RPC spans in how they should be interpreted. For example, zipkin v1 tools
 * center on RPC latency and service graphs. Root local-spans are neither
 * indicative of critical path RPC latency, nor have impact on the shape of a
 * service graph. By flagging with "lc", tools can special-case local spans.
 *
 * Zipkin v1 Spans are unqueryable unless they can be indexed by service name.
 * The only path to a service name is by (Binary)?Annotation.host.serviceName.
 * By logging "lc", a local span can be queried even if no other annotations
 * are logged.
 *
 * The value of "lc" is the namespace of Span.name. For example, it might be
 * "finatra2", for a span named "bootstrap". "lc" allows you to resolves
 * conflicts for the same Span.name, for example "finatra/bootstrap" vs
 * "finch/bootstrap". Using local component, you'd search for spans named
 * "bootstrap" where "lc=finch"
 */
const string LOCAL_COMPONENT = "lc"

#***** Annotation.value or BinaryAnnotation.key ******
/**
 * When an annotation value, this indicates when an error occurred. When a
 * binary annotation key, the value is a human readable message associated
 * with an error.
 *
 * Due to transient errors, an ERROR annotation should not be interpreted
 * as a span failure, even the annotation might explain additional latency.
 * Instrumentation should add the ERROR binary annotation when the operation
 * failed and couldn't be recovered.
 *
 * Here's an example: A span has an ERROR annotation, added when a WIRE_SEND
 * failed. Another WIRE_SEND succeeded, so there's no ERROR binary annotation
 * on the span because the overall operation succeeded.
 *
 * Note that RPC spans often include both client and server hosts: It is
 * possible that only one side perceived the error.
 */
const string ERROR = "error"

#***** BinaryAnnotation.key where value = [1] and annotation_type = BOOL ******
/**
 * Indicates a client address ("ca") in a span. Most likely, there's only one.
 * Multiple addresses are possible when a client changes its ip or port within
 * a span.
 */
const string CLIENT_ADDR = "ca"
/**
 * Indicates a server address ("sa") in a span. Most likely, there's only one.
 * Multiple addresses are possible when a client is redirected, or fails to a
 * different server ip or port.
 */
const string SERVER_ADDR = "sa"

/**
 * Indicates the network context of a service recording an annotation with two
 * exceptions.
 *
 * When a BinaryAnnotation, and key is CLIENT_ADDR or SERVER_ADDR,
 * the endpoint indicates the source or destination of an RPC. This exception
 * allows zipkin to display network context of uninstrumented services, or
 * clients such as web browsers.
 */
struct Endpoint {
  /**
   * IPv4 host address packed into 4 bytes.
   *
   * Ex for the ip 1.2.3.4, it would be (1 << 24) | (2 << 16) | (3 << 8) | 4
   */
  1: i32 ipv4
  /**
   * IPv4 port or 0, if unknown.
   *
   * Note: this is to be treated as an unsigned integer, so watch for negatives.
   */
  2: i16 port
  /**
   * Classifier of a source or destination in lowercase, such as "zipkin-web".
   *
   * This is the primary parameter for trace lookup, so should be intuitive as
   * possible, for example, matching names in service discovery.
   *
   * Conventionally, when the service name isn't known, service_name = "unknown".
   * However, it is also permissible to set service_name = "" (empty string).
   * The difference in the latter usage is that the span will not be queryable
   * by service name unless more information is added to the span with non-empty
   * service name, e.g. an additional annotation from the server.
   *
   * Particularly clients may not have a reliable service name at ingest. One
   * approach is to set service_name to "" at ingest, and later assign a
   * better label based on binary annotations, such as user agent.
   */
  3: string service_name
  /**
   * IPv6 host address packed into 16 bytes. Ex Inet6Address.getBytes()
   */
  4: optional binary ipv6
}

/**
 * Associates an event that explains latency with a timestamp.
 *
 * Unlike log statements, annotations are often codes: for example "sr".
 */
struct Annotation {
  /**
   * Microseconds from epoch.
   *
   * This value should use the most precise value possible. For example,
   * gettimeofday or multiplying currentTimeMillis by 1000.
   */
  1: i64 timestamp
  /**
   * Usually a short tag indicating an event, like "sr" or "finagle.retry".
   */
  2: string value
  /**
   * The host that recorded the value, primarily for query by service name.
   */
  3: optional Endpoint host
  // don't reuse 4: optional i32 OBSOLETE_duration         // how long did the operation take? microseconds
}

/**
 * A subset of thrift base types, except BYTES.
 */
enum AnnotationType {
  /**
   * Set to 0x01 when key is CLIENT_ADDR or SERVER_ADDR
   */
  BOOL,
  /**
   * No encoding, or type is unknown.
   */
  BYTES,
  I16,
  I32,
  I64,
  DOUBLE,
  /**
   * the only type zipkin v1 supports search against.
   */
  STRING
}

/**
 * Binary annotations are tags applied to a Span to give it context. For
 * example, a binary annotation of HTTP_PATH ("http.path") could the path
 * to a resource in a RPC call.
 *
 * Binary annotations of type STRING are always queryable, though more a
 * historical implementation detail than a structural concern.
 *
 * Binary annotations can repeat, and vary on the host. Similar to Annotation,
 * the host indicates who logged the event. This allows you to tell the
 * difference between the client and server side of the same key. For example,
 * the key "http.path" might be different on the client and server side due to
 * rewriting, like "/api/v1/myresource" vs "/myresource. Via the host field,
 * you can see the different points of view, which often help in debugging.
 */
struct BinaryAnnotation {
  /**
   * Name used to lookup spans, such as "http.path" or "finagle.version".
   */
  1: string key,
  /**
   * Serialized thrift bytes, in TBinaryProtocol format.
   *
   * For legacy reasons, byte order is big-endian. See THRIFT-3217.
   */
  2: binary value,
  /**
   * The thrift type of value, most often STRING.
   *
   * annotation_type shouldn't vary for the same key.
   */
  3: AnnotationType annotation_type,
  /**
   * The host that recorded value, allowing query by service name or address.
   *
   * There are two exceptions: when key is "ca" or "sa", this is the source or
   * destination of an RPC. This exception allows zipkin to display network
   * context of uninstrumented services, such as browsers or databases.
   */
  4: optional Endpoint host
}

/**
 * A trace is a series of spans (often RPC calls) which form a latency tree.
 *
 * Spans are usually created by instrumentation in RPC clients or servers, but
 * can also represent in-process activity. Annotations in spans are similar to
 * log statements, and are sometimes created directly by application developers
 * to indicate events of interest, such as a cache miss.
 *
 * The root span is where parent_id = Nil; it usually has the longest duration
 * in the trace.
 *
 * Span identifiers are packed into i64s, but should be treated opaquely.
 * String encoding is fixed-width lower-hex, to avoid signed interpretation.
 */
struct Span {
  /**
   * Unique 8-byte identifier for a trace, set on all spans within it.
   */
  1: i64 trace_id
  /**
   * Span name in lowercase, rpc method for example. Conventionally, when the
   * span name isn't known, name = "unknown".
   */
  3: string name,
  /**
   * Unique 8-byte identifier of this span within a trace. A span is uniquely
   * identified in storage by (trace_id, id).
   */
  4: i64 id,
  /**
   * The parent's Span.id; absent if this the root span in a trace.
   */
  5: optional i64 parent_id,
  /**
   * Associates events that explain latency with a timestamp. Unlike log
   * statements, annotations are often codes: for example SERVER_RECV("sr").
   * Annotations are sorted ascending by timestamp.
   */
  6: list<Annotation> annotations,
  /**
   * Tags a span with context, usually to support query or aggregation. For
   * example, a binary annotation key could be "http.path".
   */
  8: list<BinaryAnnotation> binary_annotations
  /**
   * True is a request to store this span even if it overrides sampling policy.
   */
  9: optional bool debug = 0
  /**
   * Epoch microseconds of the start of this span, absent if this an incomplete
   * span.
   *
   * This value should be set directly by instrumentation, using the most
   * precise value possible. For example, gettimeofday or syncing nanoTime
   * against a tick of currentTimeMillis.
   *
   * For compatibility with instrumentation that precede this field, collectors
   * or span stores can derive this via Annotation.timestamp.
   * For example, SERVER_RECV.timestamp or CLIENT_SEND.timestamp.
   *
   * Timestamp is nullable for input only. Spans without a timestamp cannot be
   * presented in a timeline: Span stores should not output spans missing a
   * timestamp.
   *
   * There are two known edge-cases where this could be absent: both cases
   * exist when a collector receives a span in parts and a binary annotation
   * precedes a timestamp. This is possible when..
   *  - The span is in-flight (ex not yet received a timestamp)
   *  - The span's start event was lost
   */
  10: optional i64 timestamp,
  /**
   * Measurement in microseconds of the critical path, if known. Durations of
   * less than one microsecond must be rounded up to 1 microsecond.
   *
   * This value should be set directly, as opposed to implicitly via annotation
   * timestamps. Doing so encourages precision decoupled from problems of
   * clocks, such as skew or NTP updates causing time to move backwards.
   *
   * For compatibility with instrumentation that precede this field, collectors
   * or span stores can derive this by subtracting Annotation.timestamp.
   * For example, SERVER_SEND.timestamp - SERVER_RECV.timestamp.
   *
   * If this field is persisted as unset, zipkin will continue to work, except
   * duration query support will be implementation-specific. Similarly, setting
   * this field non-atomically is implementation-specific.
   *
   * This field is i64 vs i32 to support spans longer than 35 minutes.
   */
  11: optional i64 duration
  /**
   * Optional unique 8-byte additional identifier for a trace. If non zero, this
   * means the trace uses 128 bit traceIds instead of 64 bit.
   */
  12: optional i64 trace_id_high
}
