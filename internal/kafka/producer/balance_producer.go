package producer

import (
	"context"
	"encoding/json"

	balancev1 "github.com/Krokozabra213/gargantua_common/gen/go/proto/balance/v1"
	apperror "github.com/GargantuaLabs/payment/internal/app_error"
	"github.com/GargantuaLabs/payment/internal/domain"
	"google.golang.org/protobuf/proto"
)

type BalanceProducer struct {
	client MessageSender
	topic  string
}

func NewBalanceProducer(client MessageSender, topic string) *BalanceProducer {
	return &BalanceProducer{
		client: client,
		topic:  topic,
	}
}

func (p *BalanceProducer) PublishBalance(ctx context.Context, balance *domain.BalanceEvent) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if balance == nil {
		return apperror.NewAppErr(apperror.CodeInternal, "BalanceEventProducer.PublishBalance",
			"balance == nil", nil, apperror.LevelError, nil)
	}

	baseFields := apperror.Fields{
		apperror.F("event_key", balance.EventKey),
	}

	var payloadData domain.BalancePayload
	if err := json.Unmarshal(balance.Payload, &payloadData); err != nil {
		return apperror.NewAppErr(apperror.CodeInternal, "BalanceEventProducer.PublishBalance",
			"failed to unmarshal balance payload", err, apperror.LevelError, baseFields)
	}

	newFields := apperror.Fields{
		apperror.F("operation_id", payloadData.OperationID),
		apperror.F("amount", payloadData.Amount),
		apperror.F("payment_id", payloadData.PaymentID),
		apperror.F("type", payloadData.Type),
		apperror.F("user_id", payloadData.UserID),
	}
	baseFields = append(baseFields, newFields...)

	var pbOpType balancev1.PaymentOpType
	switch payloadData.Type {
	case string(domain.OpTypeDeposit):
		pbOpType = balancev1.PaymentOpType_PAYMENT_OP_TYPE_DEPOSIT
	case string(domain.OpTypeRefund):
		pbOpType = balancev1.PaymentOpType_PAYMENT_OP_TYPE_REFUND
	default:
		return apperror.NewAppErr(apperror.CodeInvalidArgument, "BalanceEventProducer.PublishBalance",
			"неизвестный тип сообщения", nil, apperror.LevelError, baseFields)
	}

	pbMessage := &balancev1.PaymentOp{
		OperationId: payloadData.OperationID,
		UserId:      payloadData.UserID,
		Amount:      int32(payloadData.Amount),
		Type:        pbOpType,
		PaymentId:   payloadData.PaymentID,
	}

	dataBytes, err := proto.Marshal(pbMessage)
	if err != nil {
		return apperror.NewAppErr(apperror.CodeInternal, "BalanceEventProducer.PublishBalance",
			"failed to marshal payment op", err, apperror.LevelError, baseFields)
	}

	keyBytes := []byte(balance.EventKey)

	return p.client.SendMessage(ctx, p.topic, keyBytes, dataBytes)
}
