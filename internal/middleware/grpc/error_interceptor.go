package grpcmidware

import (
	"context"
	"log/slog"

	apperror "github.com/GargantuaLabs/payment/internal/app_error"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ErrorInterceptor struct {
	log *slog.Logger
}

func NewErrorInterceptor(log *slog.Logger) *ErrorInterceptor {
	return &ErrorInterceptor{
		log: log,
	}
}

func (i *ErrorInterceptor) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		resp, err := handler(ctx, req)
		if err == nil {
			return resp, nil
		}

		appErr := apperror.GetAppErr(err)
		if appErr == nil {
			i.log.ErrorContext(ctx, "unhandled error",
				"error", err,
			)
			return nil, status.Error(codes.Internal, "internal error")
		}

		slogLevel := apperror.SlogLevelFromAppLevel(appErr.LogLevel())
		attrs := appErr.Attrs().ToAttrs()
		if appErr.Err() != nil {
			attrs = append(attrs, slog.Any("error", appErr.Err().Error()))
		}
		i.log.LogAttrs(ctx, slogLevel, appErr.Message(), attrs...)
		return nil, status.Error(mapAppCodeToGRPC(appErr.Code()), appErr.Message())
	}
}

var codeMapping = map[apperror.Code]codes.Code{
	apperror.CodeNotFound:          codes.NotFound,
	apperror.CodeValidation:        codes.InvalidArgument,
	apperror.CodeAlreadyExists:     codes.AlreadyExists,
	apperror.CodeInsufficientFunds: codes.FailedPrecondition,
	apperror.CodeInternal:          codes.Internal,
	apperror.CodeConflict:          codes.Aborted,
}

func mapAppCodeToGRPC(code apperror.Code) codes.Code {
	if grpcCode, ok := codeMapping[code]; ok {
		return grpcCode
	}
	return codes.Internal
}
