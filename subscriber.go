package ocwatermill

import (
	"fmt"

	"github.com/pkg/errors"
	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
	"go.opencensus.io/trace"
	"go.opencensus.io/trace/propagation"

	"github.com/ThreeDotsLabs/watermill/message"

	"github.com/sagikazarmark/ocwatermill/internal"
)

type subscriberDecorator struct {
	message.Subscriber
	sub            message.Subscriber
	subscriberName string
}

func (s *subscriberDecorator) recordMetrics(msg *message.Message) {
	if msg == nil {
		return
	}

	ctx := msg.Context()

	subscriberName := message.SubscriberNameFromCtx(ctx)
	if subscriberName == "" {
		subscriberName = s.subscriberName
	}

	handlerName := message.HandlerNameFromCtx(ctx)
	if handlerName == "" {
		handlerName = tagValueNoHandler
	}

	traceContextBinary := []byte(msg.Metadata["traceContext"])
	parent, haveParent := propagation.FromBinary(traceContextBinary)

	ctx, span := trace.StartSpanWithRemoteParent(ctx, subscriberName, parent, trace.WithSampler(trace.AlwaysSample()))
	if !haveParent {
		fmt.Println("WTF NO PARENT")
	}
	// if haveParent {
	// 	span.AddLink(trace.Link{TraceID: parent.TraceID, SpanID: parent.SpanID, Type: trace.LinkTypeChild})
	// }
	msg.SetContext(ctx)

	span.AddAttributes(
		trace.StringAttribute("watermill.message_uuid", msg.UUID),
		trace.StringAttribute(HandlerAttribute, handlerName),
	)

	tags := []tag.Mutator{
		tag.Upsert(SubscriberName, subscriberName),
		tag.Upsert(HandlerName, handlerName),
	}

	go func() {
		if subscribeAlreadyObserved(ctx) {
			// decorator idempotency when applied decorator multiple times
			return
		}

		select {
		case <-msg.Acked():
			tags = append(tags, tag.Upsert(Acked, "acked"))
		case <-msg.Nacked():
			tags = append(tags, tag.Upsert(Acked, "nacked"))
		}

		span.End()
		_ = stats.RecordWithTags(ctx, tags, SubscriberReceivedMessage.M(1))
	}()

	msg.SetContext(setSubscribeObservedToCtx(msg.Context()))
}

func (s *subscriberDecorator) Close() error {
	return s.sub.Close()
}

// DecorateSubscriber decorates a publisher with instrumentation.
func DecorateSubscriber(sub message.Subscriber) (message.Subscriber, error) {
	d := &subscriberDecorator{
		sub:            sub,
		subscriberName: internal.StructName(sub),
	}

	var err error
	d.Subscriber, err = message.MessageTransformSubscriberDecorator(d.recordMetrics)(sub)
	if err != nil {
		return nil, errors.Wrap(err, "could not decorate subscriber with metrics decorator")
	}

	return d, nil
}
