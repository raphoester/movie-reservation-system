package xgrpcsrv

import (
	"context"
	"errors"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"google.golang.org/grpc"
)

func withRecoveryHandler() grpc_recovery.Option {
	return grpc_recovery.WithRecoveryHandler(func(p interface{}) error {
		return status.Errorf(codes.Internal, "caught a panic: %v", p)
	})
}

func handleCanceledRequestsInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		res, err := handler(ctx, req)
		if isContextCanceledErr(ctx.Err()) || isContextCanceledErr(err) {
			return nil, status.Errorf(codes.Canceled, "request canceled - ctx %v, err %v", ctx.Err(), err)
		}
		return res, err
	}
}

func isContextCanceledErr(err error) bool {
	if err == nil {
		return false
	}

	return errors.Is(err, context.Canceled) || strings.HasSuffix(err.Error(), "context canceled")
}

type logLevel int

const (
	logLevelDisabled logLevel = iota
	logLevelInfo
	logLevelWarn
	logLevelError
)

var grpcErrLogLevel = map[codes.Code]logLevel{
	codes.DeadlineExceeded: logLevelDisabled,
	codes.Canceled:         logLevelDisabled,
	codes.InvalidArgument:  logLevelWarn,
	codes.NotFound:         logLevelWarn,
}

func getLogLevelFromGRPCError(err error) logLevel {
	if err == nil {
		return logLevelInfo
	}
	code := status.Code(err)
	if level, ok := grpcErrLogLevel[code]; ok {
		return level
	}
	return logLevelError
}

func (l logLevel) loggerFunc(logger Logger) func(msg string, fields ...any) {
	switch l {
	case logLevelInfo:
		return logger.Info
	case logLevelWarn:
		return logger.Warn
	case logLevelError:
		return logger.Error
	default:
		return func(_ string, _ ...any) {}
	}
}

type Logger interface {
	Info(msg string, fields ...any)
	Warn(msg string, fields ...any)
	Error(msg string, fields ...any)
}

type nopLogger struct{}

func (n *nopLogger) Info(string, ...any)  {}
func (n *nopLogger) Warn(string, ...any)  {}
func (n *nopLogger) Error(string, ...any) {}

func logUnaryInterceptor(l Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		method, _ := grpc.Method(ctx)
		start := time.Now()

		// log BEFORE
		l.Info("received unary request: " + method)

		// send the request to get processed down the chain!
		res, err := handler(ctx, req)

		// log AFTER
		msg := "completed unary request: " + method
		if err != nil {
			msg = "failed unary request: " + method
		}
		getLogLevelFromGRPCError(err).
			loggerFunc(l)(
			msg,
			"status_code", status.Code(err).String(),
			"duration", time.Since(start),
			"method", method,
			"error", err,
		)

		return res, err
	}
}

func validateInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		_ *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		if validator, ok := req.(interface{ Validate() error }); ok {
			if err := validator.Validate(); err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "validation failed: %v", err)
			}
		}
		return handler(ctx, req)
	}
}
