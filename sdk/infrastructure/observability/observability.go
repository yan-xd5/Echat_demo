// Package observability 统一观测能力初始化：Trace + Metrics（OTLP）、Profiling（pprof）。
package observability

import (
	"context"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func init() {
	// 在 oteltrpc 插件 init 之前创建带真实 OTLP exporter 的 TracerProvider。
	// oteltrpc 插件会在 init 时缓存 Tracer，此后无法更换 Provider。
	ep := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if ep == "" {
		ep = "127.0.0.1:4318"
	} else {
		ep = stripScheme(ep)
	}
	svc := os.Getenv("OTEL_SERVICE_NAME")
	if svc == "" {
		svc = "echat"
	}
	res, _ := resource.New(context.Background(),
		resource.WithAttributes(
			attribute.String("service.name", svc),
		),
	)
	exp, err := otlptracehttp.New(context.Background(),
		otlptracehttp.WithEndpoint(ep),
		otlptracehttp.WithInsecure(),
	)
	if err == nil {
		otel.SetTracerProvider(sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(exp),
			sdktrace.WithResource(res),
		))
	}
}

var (
	meterProvider  *sdkmetric.MeterProvider
	tracerProvider *sdktrace.TracerProvider
	globalMeter    metric.Meter
)

// InitConfig 观测初始化配置。
type InitConfig struct {
	ServiceName    string // 服务名，如 "echat-api"
	ServiceVersion string // 版本号，如 "1.0.0"
	OTLPEndpoint   string // OTLP HTTP endpoint，默认 http://127.0.0.1:4318
	PprofPort      int    // pprof 端口，0 表示不启用
}

// Init 初始化 Metrics（OTLP HTTP exporter）+ Profiling（pprof）。
// 返回 shutdown 函数，调用方需 defer shutdown()。
func Init(cfg InitConfig) (shutdown func(context.Context) error, err error) {
	if cfg.OTLPEndpoint == "" {
		cfg.OTLPEndpoint = "http://127.0.0.1:4318"
	}
	if cfg.ServiceName == "" {
		cfg.ServiceName = "echat"
	}
	if cfg.ServiceVersion == "" {
		cfg.ServiceVersion = "1.0.0"
	}

	// 全局 TextMapPropagator（W3C TraceContext + Baggage）
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// ─── Metrics: OTLP HTTP Exporter ───
	metricExporter, err := otlpmetrichttp.New(
		context.Background(),
		otlpmetrichttp.WithEndpoint(stripScheme(cfg.OTLPEndpoint)),
		otlpmetrichttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("[Obs] 创建 OTLP metric exporter 失败: %w", err)
	}

	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			attribute.String("service.name", cfg.ServiceName),
			attribute.String("service.version", cfg.ServiceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("[Obs] 创建 Resource 失败: %w", err)
	}

	meterProvider = sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter,
			sdkmetric.WithInterval(15*time.Second),
		)),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(meterProvider)
	globalMeter = otel.Meter(cfg.ServiceName)

	// 注：TracerProvider 已在 init() 中创建（带 OTLP exporter），Init() 不再替换。
	// 各服务需在启动前设置 OTEL_SERVICE_NAME 环境变量以区分 trace 来源。

	// ─── Profiling: pprof HTTP ───
	if cfg.PprofPort > 0 {
		go startPprof(cfg.PprofPort)
		fmt.Printf("[Obs] pprof 已启动: http://127.0.0.1:%d/debug/pprof/\n", cfg.PprofPort)
	}

	fmt.Printf("[Obs] Metrics 已初始化 → %s (service=%s)\n", cfg.OTLPEndpoint, cfg.ServiceName)

	return func(ctx context.Context) error {
		if meterProvider != nil {
			return meterProvider.Shutdown(ctx)
		}
		return nil
	}, nil
}

// Meter 返回全局 Meter，用于创建自定义指标。
func Meter() metric.Meter { return globalMeter }

// ─── HTTP 状态码传递（Filter → writeJSON → Filter） ───

type httpStatusCtxKey struct{}

type httpStatusHolder struct {
	code int
}

// WithHTTPStatusTracker 注入状态码 holder 到 context（由 metrics filter 调用）。
func WithHTTPStatusTracker(ctx context.Context) context.Context {
	return context.WithValue(ctx, httpStatusCtxKey{}, &httpStatusHolder{})
}

// SetHTTPStatus 将 HTTP 状态码写入 holder（由 writeJSON 调用）。
func SetHTTPStatus(ctx context.Context, code int) {
	if h, ok := ctx.Value(httpStatusCtxKey{}).(*httpStatusHolder); ok {
		h.code = code
	}
}

// HTTPStatusFromContext 从 context 读取 HTTP 状态码（由 metrics filter 调用）。
func HTTPStatusFromContext(ctx context.Context) int {
	if h, ok := ctx.Value(httpStatusCtxKey{}).(*httpStatusHolder); ok {
		return h.code
	}
	return 0
}

// TraceIDFromContext 从 context 提取 Trace ID。
func TraceIDFromContext(ctx context.Context) string {
	sc := trace.SpanContextFromContext(ctx)
	if sc.HasTraceID() {
		return sc.TraceID().String()
	}
	return ""
}

// SpanIDFromContext 从 context 提取 Span ID。
func SpanIDFromContext(ctx context.Context) string {
	sc := trace.SpanContextFromContext(ctx)
	if sc.HasSpanID() {
		return sc.SpanID().String()
	}
	return ""
}

// startPprof 启动 pprof HTTP 端点。
func startPprof(port int) {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Printf("[Obs] pprof 启动失败: %v\n", err)
	}
}

// stripScheme 去除 http:// 前缀，适配 otel exporter。
func stripScheme(endpoint string) string {
	if len(endpoint) > 7 && endpoint[:7] == "http://" {
		return endpoint[7:]
	}
	if len(endpoint) > 8 && endpoint[:8] == "https://" {
		return endpoint[8:]
	}
	return endpoint
}
