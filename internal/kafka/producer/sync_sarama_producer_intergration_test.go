//go:build integration

package producer

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/GargantuaLabs/payment/internal/config"
	"github.com/IBM/sarama"
	"github.com/stretchr/testify/require"

	"github.com/testcontainers/testcontainers-go/modules/kafka"
	kafkamodule "github.com/testcontainers/testcontainers-go/modules/kafka"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestSaramaSyncProducer_SendMessage(t *testing.T) {
	ctx := context.Background()

	kafkaContainer, err := kafkamodule.Run(
		ctx,
		"confluentinc/confluent-local:7.5.0",
		kafka.WithClusterID("test-cluster"),
	)
	require.NoError(t, err)

	defer func() {
		_ = kafkaContainer.Terminate(ctx)
	}()

	brokers, err := kafkaContainer.Brokers(ctx)
	require.NoError(t, err)

	kafkaVersion, err := sarama.ParseKafkaVersion("4.1.2")
	require.NoError(t, err)

	adminCfg := sarama.NewConfig()
	adminCfg.Version = kafkaVersion

	admin, err := sarama.NewClusterAdmin(
		brokers,
		adminCfg,
	)
	require.NoError(t, err)

	defer func() {
		require.NoError(t, admin.Close())
	}()

	topic := "payments-test"

	err = admin.CreateTopic(
		topic,
		&sarama.TopicDetail{
			NumPartitions:     1,
			ReplicationFactor: 1,
		},
		false,
	)
	require.NoError(t, err)

	producerCfg := config.ProducerConfig{
		Brokers: brokers,

		Version:  "4.1.2",
		ClientID: "producer-it-test",

		MaxOpenRequests: 1,
		MaxRetries:      3,
		RetryBackoff:    100 * time.Millisecond,

		DialTimeout:  5 * time.Second,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	producer, err := NewSaramaSyncProducer(
		testLogger(),
		producerCfg,
	)
	require.NoError(t, err)

	defer func() {
		require.NoError(t, producer.Close())
	}()

	consumerCfg := sarama.NewConfig()
	consumerCfg.Version = kafkaVersion

	consumer, err := sarama.NewConsumer(
		brokers,
		consumerCfg,
	)
	require.NoError(t, err)

	defer func() {
		require.NoError(t, consumer.Close())
	}()

	partitionConsumer, err := consumer.ConsumePartition(
		topic,
		0,
		sarama.OffsetNewest,
	)
	require.NoError(t, err)

	defer func() {
		require.NoError(t, partitionConsumer.Close())
	}()

	key := []byte("payment-123")
	value := []byte(`{"status":"completed"}`)

	err = producer.SendMessage(
		ctx,
		topic,
		key,
		value,
	)
	require.NoError(t, err)

	select {
	case msg := <-partitionConsumer.Messages():

		require.Equal(t, string(key), string(msg.Key))
		require.Equal(t, string(value), string(msg.Value))

		require.Len(t, msg.Headers, 1)

		require.Equal(
			t,
			[]byte("content-type"),
			msg.Headers[0].Key,
		)

		require.Equal(
			t,
			[]byte("application/json"),
			msg.Headers[0].Value,
		)

	case <-time.After(15 * time.Second):
		t.Fatal("message was not received from kafka")
	}
}
