package producer

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	balancev1 "github.com/Krokozabra213/gargantua_common/gen/go/proto/balance/v1"
	"github.com/GargantuaLabs/payment/internal/domain"
	"github.com/GargantuaLabs/payment/internal/kafka/producer/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/proto"
)

func validBalanceEvent() *domain.BalanceEvent {
	payload := domain.BalancePayload{
		OperationID: "op-123",
		UserID:      "user-42",
		PaymentID:   "payment-777",
		Type:        string(domain.OpTypeDeposit),
		Amount:      1000,
	}

	data, _ := json.Marshal(payload)

	return &domain.BalanceEvent{
		EventKey: "event-key",
		Payload:  data,
	}
}

func TestPublishBalance_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mocks.NewMockMessageSender(ctrl)

	producer := NewBalanceProducer(
		client,
		"payments",
	)

	event := validBalanceEvent()

	client.EXPECT().
		SendMessage(
			gomock.Any(),
			"payments",
			[]byte("event-key"),
			gomock.Any(),
		).
		DoAndReturn(func(
			_ context.Context,
			_ string,
			_ []byte,
			value []byte,
		) error {
			var msg balancev1.PaymentOp

			err := proto.Unmarshal(value, &msg)
			require.NoError(t, err)

			require.Equal(t, "op-123", msg.OperationId)
			require.Equal(t, "user-42", msg.UserId)
			require.Equal(t, "payment-777", msg.PaymentId)
			require.Equal(t, int32(1000), msg.Amount)
			require.Equal(
				t,
				balancev1.PaymentOpType_PAYMENT_OP_TYPE_DEPOSIT,
				msg.Type,
			)

			return nil
		})

	err := producer.PublishBalance(context.Background(), event)

	require.NoError(t, err)
}

func TestPublishBalance_NilBalance(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mocks.NewMockMessageSender(ctrl)

	producer := NewBalanceProducer(
		client,
		"payments",
	)

	err := producer.PublishBalance(
		context.Background(),
		nil,
	)

	require.Error(t, err)
}

func TestPublishBalance_ContextCanceled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mocks.NewMockMessageSender(ctrl)

	producer := NewBalanceProducer(
		client,
		"payments",
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := producer.PublishBalance(
		ctx,
		validBalanceEvent(),
	)

	require.ErrorIs(t, err, context.Canceled)
}

func TestPublishBalance_InvalidJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mocks.NewMockMessageSender(ctrl)

	producer := NewBalanceProducer(
		client,
		"payments",
	)

	event := &domain.BalanceEvent{
		EventKey: "key",
		Payload:  []byte("not-json"),
	}

	err := producer.PublishBalance(
		context.Background(),
		event,
	)

	require.Error(t, err)
}

func TestPublishBalance_UnknownOperationType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mocks.NewMockMessageSender(ctrl)

	producer := NewBalanceProducer(
		client,
		"payments",
	)

	payload := domain.BalancePayload{
		OperationID: "op-123",
		UserID:      "user",
		PaymentID:   "payment",
		Type:        "unknown_type",
		Amount:      100,
	}

	data, _ := json.Marshal(payload)

	event := &domain.BalanceEvent{
		EventKey: "key",
		Payload:  data,
	}

	err := producer.PublishBalance(
		context.Background(),
		event,
	)

	require.Error(t, err)
}

func TestPublishBalance_SendMessageError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mocks.NewMockMessageSender(ctrl)

	producer := NewBalanceProducer(
		client,
		"payments",
	)

	kafkaErr := errors.New("kafka unavailable")

	client.EXPECT().
		SendMessage(
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
		).
		Return(kafkaErr)

	err := producer.PublishBalance(
		context.Background(),
		validBalanceEvent(),
	)

	require.ErrorIs(t, err, kafkaErr)
}

func TestPublishBalance_OperationTypeMapping(t *testing.T) {
	tests := []struct {
		name     string
		opType   string
		expected balancev1.PaymentOpType
	}{
		{
			name:     "deposit",
			opType:   string(domain.OpTypeDeposit),
			expected: balancev1.PaymentOpType_PAYMENT_OP_TYPE_DEPOSIT,
		},
		{
			name:     "refund",
			opType:   string(domain.OpTypeRefund),
			expected: balancev1.PaymentOpType_PAYMENT_OP_TYPE_REFUND,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			client := mocks.NewMockMessageSender(ctrl)

			producer := NewBalanceProducer(
				client,
				"payments",
			)

			payload := domain.BalancePayload{
				OperationID: "op-123",
				UserID:      "user-42",
				PaymentID:   "payment-777",
				Type:        tt.opType,
				Amount:      1000,
			}

			data, err := json.Marshal(payload)
			require.NoError(t, err)

			event := &domain.BalanceEvent{
				EventKey: "event-key",
				Payload:  data,
			}

			client.EXPECT().
				SendMessage(
					gomock.Any(),
					"payments",
					[]byte("event-key"),
					gomock.Any(),
				).
				DoAndReturn(func(
					_ context.Context,
					_ string,
					_ []byte,
					value []byte,
				) error {
					var msg balancev1.PaymentOp

					err := proto.Unmarshal(value, &msg)
					require.NoError(t, err)

					require.Equal(t, "op-123", msg.OperationId)
					require.Equal(t, "user-42", msg.UserId)
					require.Equal(t, "payment-777", msg.PaymentId)
					require.Equal(t, int32(1000), msg.Amount)
					require.Equal(t, tt.expected, msg.Type)

					return nil
				})

			err = producer.PublishBalance(
				context.Background(),
				event,
			)

			require.NoError(t, err)
		})
	}
}
