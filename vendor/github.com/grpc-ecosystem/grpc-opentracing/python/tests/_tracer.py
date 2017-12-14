from collections import defaultdict
import enum

import opentracing


@enum.unique
class SpanRelationship(enum.Enum):
    NONE = 0
    FOLLOWS_FROM = 1
    CHILD_OF = 2


class _SpanContext(opentracing.SpanContext):

    def __init__(self, identity):
        self.identity = identity


class _Span(opentracing.Span):

    def __init__(self, tracer, identity, tags=None):
        super(_Span, self).__init__(tracer, _SpanContext(identity))
        self._identity = identity
        if tags is None:
            tags = {}
        self._tags = tags

    def set_tag(self, key, value):
        self._tags[key] = value

    def get_tag(self, key):
        return self._tags.get(key, None)


class Tracer(opentracing.Tracer):

    def __init__(self):
        super(Tracer, self).__init__()
        self._counter = 0
        self._spans = {}
        self._relationships = defaultdict(lambda: None)

    def start_span(self,
                   operation_name=None,
                   child_of=None,
                   references=None,
                   tags=None,
                   start_time=None):
        identity = self._counter
        self._counter += 1
        if child_of is not None:
            self._relationships[(child_of.identity,
                                 identity)] = opentracing.ReferenceType.CHILD_OF
        if references is not None:
            assert child_of is None and len(
                references) == 1, 'Only a single reference is supported'
            reference_type, span_context = references[0]
            self._relationships[(span_context.identity,
                                 identity)] = reference_type
        span = _Span(self, identity, tags)
        self._spans[identity] = span
        return span

    def inject(self, span_context, format, carrier):
        if format != opentracing.Format.HTTP_HEADERS and isinstance(carrier,
                                                                    dict):
            raise opentracing.UnsupportedFormatException(format)
        carrier['span-identity'] = str(span_context.identity)

    def extract(self, format, carrier):
        if format != opentracing.Format.HTTP_HEADERS and isinstance(carrier,
                                                                    dict):
            raise opentracing.UnsupportedFormatException(format)
        if 'span-identity' not in carrier:
            raise opentracing.SpanContextCorruptedException
        return _SpanContext(int(carrier['span-identity']))

    def get_relationship(self, identity1, identity2):
        return self._relationships[(identity1, identity2)]

    def get_span(self, identity):
        return self._spans.get(identity, None)
