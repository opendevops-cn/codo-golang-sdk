package redis

import (
	"os"
	"strconv"
	"strings"
	"time"

	redisotelv8 "github.com/go-redis/redis/extra/redisotel/v8"
	redisv8 "github.com/go-redis/redis/v8"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

type RedisConfigOption func(*RedisConfig)

type RedisConfig struct {
	Host         string `json:"host" yaml:"host" env:"DEFAULT_REDIS_HOST"`
	Port         uint32 `json:"port" yaml:"port" env:"DEFAULT_REDIS_PORT"`
	Pass         string `json:"pass" yaml:"pass" env:"DEFAULT_REDIS_PASS"`
	DialTimeout  uint32 `json:"dial_timeout" yaml:"dialTimeout" env:"DEFAULT_REDIS_DIAL_TIMEOUT"`
	ReadTimeout  uint32 `json:"read_timeout" yaml:"readTimeout" env:"DEFAULT_REDIS_READ_TIMEOUT"`
	WriteTimeout uint32 `json:"write_timeout" yaml:"writeTimeout" env:"DEFAULT_REDIS_WRITE_TIMEOUT"`
	DB           uint32 `json:"db" yaml:"db" env:"DEFAULT_REDIS_DB"`

	TracerProvider trace.TracerProvider `json:"-"`
	MeterProvider  metric.MeterProvider `json:"-"`
}

func defaultRedisConfig() RedisConfig {
	emptyOr := func(str string, defaultVal string) string {
		str = strings.TrimSpace(str)
		if str == "" {
			return defaultVal
		}
		return str
	}
	host := emptyOr(os.Getenv("DEFAULT_REDIS_HOST"), "127.0.0.1")
	port := emptyOr(os.Getenv("DEFAULT_REDIS_PORT"), "6379")
	pass := emptyOr(os.Getenv("DEFAULT_REDIS_PASS"), "123456")
	dialTimeout := emptyOr(os.Getenv("DEFAULT_REDIS_DIAL_TIMEOUT"), "10")
	readTimeout := emptyOr(os.Getenv("DEFAULT_REDIS_READ_TIMEOUT"), "10")
	writeTimeout := emptyOr(os.Getenv("DEFAULT_REDIS_WRITE_TIMEOUT"), "10")
	db := emptyOr(os.Getenv("DEFAULT_REDIS_DB"), "0")

	i64Port, _ := strconv.Atoi(port)
	i64DialTimeout, _ := strconv.Atoi(dialTimeout)
	i64ReadTimeout, _ := strconv.Atoi(readTimeout)
	i64WriteTimeout, _ := strconv.Atoi(writeTimeout)
	i64DB, _ := strconv.Atoi(db)

	return RedisConfig{
		Host:           host,
		Port:           uint32(i64Port),
		Pass:           pass,
		DialTimeout:    uint32(i64DialTimeout),
		ReadTimeout:    uint32(i64ReadTimeout),
		WriteTimeout:   uint32(i64WriteTimeout),
		DB:             uint32(i64DB),
		TracerProvider: otel.GetTracerProvider(),
		MeterProvider:  otel.GetMeterProvider(),
	}
}

func NewRedis(opts ...RedisConfigOption) (*redis.Client, error) {
	cfg := defaultRedisConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	addr := cfg.Host + ":" + strconv.Itoa(int(cfg.Port))
	redisClient := redis.NewClient(&redis.Options{
		Network:      "tcp",
		Addr:         addr,
		Password:     cfg.Pass,
		DialTimeout:  time.Duration(cfg.ReadTimeout) * time.Second,
		ReadTimeout:  time.Duration(cfg.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.WriteTimeout) * time.Second,
		DB:           int(cfg.DB),
	})

	// Enable tracing instrumentation.
	if err := redisotel.InstrumentTracing(redisClient,
		redisotel.WithTracerProvider(cfg.TracerProvider),
		redisotel.WithAttributes(semconv.DBSystemRedis),
	); err != nil {
		return nil, err
	}

	// Enable metrics instrumentation.
	if err := redisotel.InstrumentMetrics(redisClient,
		redisotel.WithMeterProvider(cfg.MeterProvider),
		redisotel.WithAttributes(semconv.DBSystemRedis),
	); err != nil {
		return nil, err
	}
	return redisClient, nil
}

func NewRedisV8(opts ...RedisConfigOption) (*redisv8.Client, error) {
	cfg := defaultRedisConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	addr := cfg.Host + ":" + strconv.Itoa(int(cfg.Port))
	redisClient := redisv8.NewClient(&redisv8.Options{
		Network:      "tcp",
		Addr:         addr,
		Password:     cfg.Pass,
		DialTimeout:  time.Duration(cfg.ReadTimeout) * time.Second,
		ReadTimeout:  time.Duration(cfg.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.WriteTimeout) * time.Second,
	})

	// Enable tracing instrumentation.
	redisClient.AddHook(redisotelv8.NewTracingHook(
		redisotelv8.WithTracerProvider(cfg.TracerProvider),
		redisotelv8.WithAttributes(semconv.DBSystemRedis),
	))
	return redisClient, nil
}
