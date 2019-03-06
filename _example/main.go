package main

import (
	"context"
	"errors"
	"flag"
	"math"
	"math/rand"
	"net/http"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/message/infrastructure/gochannel"
	"github.com/ThreeDotsLabs/watermill/message/router/middleware"
	"github.com/ThreeDotsLabs/watermill/message/router/plugin"
	"github.com/go-chi/chi"
	"go.opencensus.io/exporter/prometheus"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"

	"github.com/sagikazarmark/ocwatermill"
)

var (
	metricsAddr  = flag.String("metrics", ":8081", "The address that will expose /metrics for Prometheus")
	handlerDelay = flag.Float64("delay", 0, "The stdev of normal distribution of delay in handler (in seconds), to simulate load")

	logger = watermill.NewStdLogger(true, true)
	random = rand.New(rand.NewSource(time.Now().Unix()))
)

var (
	PublisherPublishTimeView = &view.View{
		Name:        "publish_time_seconds",
		Description: "The time that a publishing attempt (success or not) took",
		Measure:     ocwatermill.PublisherPublishTime,
		TagKeys:     []tag.Key{ocwatermill.PublisherName, ocwatermill.HandlerName, ocwatermill.Success},
		Aggregation: ocwatermill.DefaultMillisecondsDistribution,
	}

	SubscriberReceivedMessageView = &view.View{
		Name:        "subscriber_messages_received_total",
		Description: "Number of messages received by the subscriber",
		Measure:     ocwatermill.SubscriberReceivedMessage,
		TagKeys:     []tag.Key{ocwatermill.SubscriberName, ocwatermill.HandlerName, ocwatermill.Acked},
		Aggregation: view.Count(),
	}

	HandlerExecutionTimeView = &view.View{
		Name:        "handler_execution_time_seconds",
		Description: "The total time elapsed while executing the handler function in seconds",
		Measure:     ocwatermill.HandlerExecutionTime,
		TagKeys:     []tag.Key{ocwatermill.HandlerName, ocwatermill.Success},
		Aggregation: ocwatermill.DefaultHandlerExecutionTimeDistribution,
	}
)

func createPrometheusExporterHandler(exporter *prometheus.Exporter, addr string) (cancel func()) {
	router := chi.NewRouter()

	router.Get("/metrics", func(w http.ResponseWriter, r *http.Request) {
		exporter.ServeHTTP(w, r)
	})
	server := http.Server{
		Addr:    addr,
		Handler: router,
	}

	go func() {
		err := server.ListenAndServe()
		if err != nil {
			panic(err)
		}
	}()

	wait := make(chan struct{})
	go func() {
		<-wait
		server.Close()
	}()

	return func() { close(wait) }
}

func delay() {
	seconds := *handlerDelay
	if seconds == 0 {
		return
	}
	delay := math.Abs(random.NormFloat64() * seconds)
	time.Sleep(time.Duration(float64(time.Second) * delay))
}

// handler publishes 0-4 messages with a random delay.
func handler(msg *message.Message) ([]*message.Message, error) {
	delay()

	numOutgoing := random.Intn(4)
	outgoing := make([]*message.Message, numOutgoing)

	for i := 0; i < numOutgoing; i++ {
		outgoing[i] = msg.Copy()
	}
	return outgoing, nil
}

// consumeMessages consumes the messages exiting the handler.
func consumeMessages(subscriber message.Subscriber) {
	messages, err := subscriber.Subscribe(context.Background(), "pub_topic")
	if err != nil {
		panic(err)
	}

	for msg := range messages {
		msg.Ack()
	}
}

// produceMessages produces the incoming messages in delays of 50-100 milliseconds.
func produceMessages(routerClosed chan struct{}, publisher message.Publisher) {
	for {
		select {
		case <-routerClosed:
			return
		default:
			// go on
		}

		time.Sleep(50*time.Millisecond + time.Duration(random.Intn(50))*time.Millisecond)
		msg := message.NewMessage(watermill.NewUUID(), []byte{})
		_ = publisher.Publish("sub_topic", msg)
	}
}

func main() {
	flag.Parse()

	pubSub := gochannel.NewGoChannel(gochannel.Config{}, logger)

	router, err := message.NewRouter(
		message.RouterConfig{},
		logger,
	)
	if err != nil {
		panic(err)
	}

	err = view.Register(
		PublisherPublishTimeView,
		SubscriberReceivedMessageView,
		HandlerExecutionTimeView,
	)
	if err != nil {
		panic(err)
	}

	exporter, err := prometheus.NewExporter(prometheus.Options{})
	if err != nil {
		panic(err)
	}
	view.RegisterExporter(exporter)

	closeMetricsServer := createPrometheusExporterHandler(exporter, *metricsAddr)
	defer closeMetricsServer()

	// we leave the namespace and subsystem empty
	ocwatermill.Register(router)

	router.AddMiddleware(
		middleware.Recoverer,
		middleware.RandomFail(0.1),
		middleware.RandomPanic(0.1),
	)
	router.AddPlugin(plugin.SignalsHandler)

	router.AddHandler(
		"metrics-example",
		"sub_topic",
		pubSub,
		"pub_topic",
		pubSub,
		handler,
	)

	// separate the publisher from pubSub to decorate it separately
	sub := pubSub.(message.Subscriber)
	pub := randomFailPublisherDecorator{pubSub, 0.1}

	// The handler's publisher and subscriber will be decorated by `AddPrometheusRouterMetrics`.
	// We are using the same pub/sub to generate messages incoming to the handler
	// and consume the outgoing messages.
	// They will have `handler_name=<no handler>` label in Prometheus.
	subWithMetrics, err := ocwatermill.DecorateSubscriber(sub)
	if err != nil {
		panic(err)
	}
	pubWithMetrics, err := ocwatermill.DecoratePublisher(pub)
	if err != nil {
		panic(err)
	}

	routerClosed := make(chan struct{})
	go produceMessages(routerClosed, pubWithMetrics)
	go consumeMessages(subWithMetrics)

	_ = router.Run()
	close(routerClosed)
}

type randomFailPublisherDecorator struct {
	message.Publisher
	failProbability float64
}

func (r randomFailPublisherDecorator) Publish(topic string, messages ...*message.Message) error {
	if random.Float64() < r.failProbability {
		return errors.New("random publishing failure")
	}
	return r.Publisher.Publish(topic, messages...)
}
