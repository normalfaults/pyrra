package prometheus

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bufbuild/connect-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Interceptor struct {
	clientStarted  *prometheus.CounterVec
	clientHandled  *prometheus.CounterVec
	clientDuration *prometheus.HistogramVec

	serverStarted  *prometheus.CounterVec
	serverHandled  *prometheus.CounterVec
	serverDuration *prometheus.HistogramVec
}

func NewInterceptor(reg prometheus.Registerer) *Interceptor {
	labelCode := "grpc_code"
	labelMethod := "grpc_method"
	labelService := "grpc_service"
	labelType := "grpc_type"

	clientStarted := promauto.With(reg).NewCounterVec(prometheus.CounterOpts{
		Name: "grpc_client_started_total",
		Help: "Total number of RPCs started on the client.",
	}, []string{
		labelMethod,
		labelService,
		labelType,
	})
	clientHandled := promauto.With(reg).NewCounterVec(prometheus.CounterOpts{
		Name: "grpc_client_handled_total",
		Help: "Total number of RPCs completed by the client, regardless of success or failure.",
	}, []string{
		labelCode,
		labelMethod,
		labelService,
		labelType,
	})
	clientDuration := promauto.With(reg).NewHistogramVec(prometheus.HistogramOpts{
		Name:    "grpc_client_handling_seconds_bucket",
		Help:    "Histogram of response latency (seconds) of gRPC that had been application-level handled by the server.",
		Buckets: prometheus.DefBuckets,
	}, []string{
		labelMethod,
		labelService,
		labelType,
	})

	serverStarted := promauto.With(reg).NewCounterVec(prometheus.CounterOpts{
		Name: "grpc_server_started_total",
		Help: "Total number of RPCs started on the server.",
	}, []string{
		labelMethod,
		labelService,
		labelType,
	})
	serverHandled := promauto.With(reg).NewCounterVec(prometheus.CounterOpts{
		Name: "grpc_server_handled_total",
		Help: "The amount of requests handled per connect service and method by code",
	}, []string{
		labelCode,
		labelMethod,
		labelService,
		labelType,
	})
	serverDuration := promauto.With(reg).NewHistogramVec(prometheus.HistogramOpts{
		Name:    "grpc_server_handling_seconds_bucket",
		Help:    "Histogram of response latency (seconds) of gRPC that had been application-level handled by the server.",
		Buckets: prometheus.DefBuckets,
	}, []string{
		labelMethod,
		labelService,
		labelType,
	})

	return &Interceptor{
		clientHandled:  clientHandled,
		clientStarted:  clientStarted,
		clientDuration: clientDuration,

		serverStarted:  serverStarted,
		serverHandled:  serverHandled,
		serverDuration: serverDuration,
	}
}

func (i *Interceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		start := time.Now()
		spec := req.Spec()
		procedure := strings.Split(spec.Procedure, "/")
		if len(procedure) != 3 {
			return nil, connect.NewError(
				connect.CodeInternal,
				fmt.Errorf("procedure in prometheus interceptor malformed: %s", spec.Procedure),
			)
		}
		service, method := procedure[1], procedure[2]

		if spec.IsClient {
			i.clientStarted.WithLabelValues(
				method,
				service,
				streamType(spec.StreamType),
			)
		} else {
			i.serverStarted.WithLabelValues(
				method,
				service,
				streamType(spec.StreamType),
			).Inc()
		}

		resp, err := next(ctx, req)

		if spec.IsClient {
			i.clientDuration.WithLabelValues(
				method,
				service,
				streamType(spec.StreamType),
			).Observe(time.Since(start).Seconds())
			i.clientHandled.WithLabelValues(
				code(err),
				method,
				service,
				streamType(spec.StreamType),
			).Inc()
		} else { // server
			i.serverDuration.WithLabelValues(
				method,
				service,
				streamType(spec.StreamType),
			).Observe(time.Since(start).Seconds())
			i.serverHandled.WithLabelValues(
				code(err),
				method,
				service,
				streamType(spec.StreamType),
			).Inc()
		}

		return resp, err
	}
}

func code(err error) string {
	if err == nil {
		return "ok"
	}
	return connect.CodeOf(err).String()
}

func streamType(t connect.StreamType) string {
	switch t {
	case connect.StreamTypeUnary:
		return "unary"
	case connect.StreamTypeClient:
		return "client_stream"
	case connect.StreamTypeServer:
		return "server_stream"
	case connect.StreamTypeBidi:
		return "bidi_stream"
	default:
		return ""
	}
}

func (i *Interceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	// TODO implement me
	panic("implement me")
}

func (i *Interceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	// TODO implement me
	panic("implement me")
}
