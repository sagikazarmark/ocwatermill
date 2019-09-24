package ocwatermill

import (
	"github.com/ThreeDotsLabs/watermill/message"
)

func Register(r *message.Router) {
	r.AddPublisherDecorators(PublisherDecorator())
	r.AddSubscriberDecorators(DecorateSubscriber)
	r.AddMiddleware(Middleware)
}
