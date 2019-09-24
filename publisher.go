package ocwatermill

import (
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
	"go.opencensus.io/trace"
	"go.opencensus.io/trace/propagation"

	"github.com/ThreeDotsLabs/watermill/message"

	"github.com/sagikazarmark/ocwatermill/internal"
)

type publisherDecorator struct {
	pub           message.Publisher
	publisherName string
	startOptions  trace.StartOptions
}

func (p *publisherDecorator) Publish(topic string, messages ...*message.Message) (err error) {
	if len(messages) == 0 {
		return p.pub.Publish(topic)
	}

	// TODO: take ctx not only from first msg. Might require changing the signature of Publish, which is planned anyway.
	ctx := messages[0].Context()

	publisherName := message.PublisherNameFromCtx(ctx)
	if publisherName == "" {
		publisherName = p.publisherName
	}

	handlerName := message.HandlerNameFromCtx(ctx)
	if handlerName == "" {
		handlerName = tagValueNoHandler
	}

	ctx, span := trace.StartSpan(ctx, publisherName, trace.WithSampler(p.startOptions.Sampler))

	span.AddAttributes(
		trace.StringAttribute("watermill.message_uuid", messages[0].UUID),
		trace.StringAttribute(TopicAttribute, topic),
		trace.StringAttribute(HandlerAttribute, handlerName),
	)

	tags := []tag.Mutator{
		tag.Upsert(PublisherName, publisherName),
		tag.Upsert(HandlerName, handlerName),
	}

	traceContextBinary := propagation.Binary(span.SpanContext())

	for _, msg := range messages {
		msg.Metadata["traceContext"] = string(traceContextBinary)
	}

	start := time.Now()

	defer func() {
		if publishAlreadyObserved(ctx) {
			// decorator idempotency when applied decorator multiple times
			return
		}

		if err != nil {
			span.SetStatus(trace.Status{Code: trace.StatusCodeUnknown, Message: err.Error()})

			tags = append(tags, tag.Upsert(Success, "false"))
		} else {
			span.SetStatus(trace.Status{Code: trace.StatusCodeOK})

			tags = append(tags, tag.Upsert(Success, "true"))
		}

		span.End()
		_ = stats.RecordWithTags(ctx, tags, PublisherPublishTime.M(float64(time.Since(start))/float64(time.Millisecond)))
	}()

	for _, msg := range messages {
		msg.SetContext(setPublishObservedToCtx(msg.Context()))
	}

	return p.pub.Publish(topic, messages...)
}

func (p *publisherDecorator) Close() error {
	return p.pub.Close()
}

type PublisherDecoratorOption interface {
	apply(pubd *publisherDecorator)
}

type publisherDecoratorOptionFunc func(pubd *publisherDecorator)

func (fn publisherDecoratorOptionFunc) apply(pubd *publisherDecorator) {
	fn(pubd)
}

func TraceStartOptions(startOptions trace.StartOptions) PublisherDecoratorOption {
	return publisherDecoratorOptionFunc(func(pubd *publisherDecorator) {
		pubd.startOptions = startOptions
	})
}

// PublisherDecorator creates a publisher decorator that instruments a publisher.
func PublisherDecorator(opts ...PublisherDecoratorOption) message.PublisherDecorator {
	return func(pub message.Publisher) (message.Publisher, error) {
		d := &publisherDecorator{
			pub:           pub,
			publisherName: internal.StructName(pub),
		}

		for _, opt := range opts {
			opt.apply(d)
		}

		return d, nil
	}
}
