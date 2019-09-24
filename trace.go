package ocwatermill

// Attributes recorded on the span for the requests.
// Only trace exporters will need them.
const (
	TopicAttribute   = "watermill.topic"
	HandlerAttribute = "watermill.handler"
)
