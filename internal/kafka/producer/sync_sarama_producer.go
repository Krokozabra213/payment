package producer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/GargantuaLabs/payment/internal/config"
	"github.com/IBM/sarama"
)

//go:generate mockgen -source=sync_sarama_producer.go -destination=mocks/mock_producer.go -package=mocks

type MessageSender interface {
	SendMessage(ctx context.Context, topic string, key []byte, value []byte) error
}

type Producer interface {
	MessageSender
	Close() error
}

type SaramaSyncProducer struct {
	p      sarama.SyncProducer
	logger *slog.Logger
}

func NewSaramaSyncProducer(
	logger *slog.Logger,
	producerCfg config.ProducerConfig,
) (*SaramaSyncProducer, error) {
	cfg := sarama.NewConfig()

	v, err := sarama.ParseKafkaVersion(producerCfg.Version)
	if err != nil {
		return nil, fmt.Errorf("parse kafka version: %w", err)
	}
	cfg.Version = v
	cfg.ClientID = producerCfg.ClientID

	cfg.Producer.RequiredAcks = sarama.WaitForAll
	cfg.Producer.Return.Successes = true
	cfg.Producer.Retry.Max = producerCfg.MaxRetries
	cfg.Producer.Retry.Backoff = producerCfg.RetryBackoff

	cfg.Producer.Idempotent = true
	cfg.Net.MaxOpenRequests = producerCfg.MaxOpenRequests

	cfg.Net.DialTimeout = producerCfg.DialTimeout
	cfg.Net.ReadTimeout = producerCfg.ReadTimeout
	cfg.Net.WriteTimeout = producerCfg.WriteTimeout

	cfg.Producer.Compression = sarama.CompressionZSTD

	if cfg.Producer.Idempotent && cfg.Net.MaxOpenRequests != 1 {
		return nil, errors.New("idempotent producer requires MaxOpenRequests=1")
	}

	p, err := sarama.NewSyncProducer(producerCfg.Brokers, cfg)
	if err != nil {
		return nil, fmt.Errorf("create sync producer: %w", err)
	}

	return &SaramaSyncProducer{
		p:      p,
		logger: logger,
	}, nil
}

func (p *SaramaSyncProducer) SendMessage(
	ctx context.Context,
	topic string,
	key, value []byte,
) error {
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.ByteEncoder(key),
		Value: sarama.ByteEncoder(value),
		Headers: []sarama.RecordHeader{
			{Key: []byte("content-type"), Value: []byte("application/json")},
		},
	}

	_, _, err := p.p.SendMessage(msg)
	if err != nil {
		return err
	}
	return nil
}

func (p *SaramaSyncProducer) Close() error {
	return p.p.Close()
}
