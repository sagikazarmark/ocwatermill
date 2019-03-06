package ocwatermill

import (
	"github.com/ThreeDotsLabs/watermill/message"
)

func DecoratePubSub(pubSub message.PubSub) (message.PubSub, error) {
	pub, err := DecoratePublisher(pubSub)
	if err != nil {
		return nil, err
	}

	sub, err := DecorateSubscriber(pubSub)
	if err != nil {
		return nil, err
	}

	return message.NewPubSub(pub, sub), nil
}
