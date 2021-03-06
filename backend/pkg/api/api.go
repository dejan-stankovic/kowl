package api

import (
	"github.com/Shopify/sarama"
	"github.com/cloudhut/common/logging"
	"github.com/cloudhut/common/rest"
	"github.com/cloudhut/kowl/backend/pkg/kafka"
	"github.com/cloudhut/kowl/backend/pkg/owl"
	"github.com/prometheus/common/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// API represents the server and all it's dependencies to serve incoming user requests
type API struct {
	Cfg *Config

	Logger   *zap.Logger
	KafkaSvc *kafka.Service
	OwlSvc   *owl.Service

	Hooks *Hooks // Hooks to add additional functionality from the outside at different places (used by Kafka Owl Business)
}

// New creates a new API instance
func New(cfg *Config) *API {
	logger := logging.NewLogger(&cfg.Logger, cfg.MetricsNamespace)

	// Create separate logger for sarama
	saramaLogger, err := zap.NewStdLogAt(logger.With(zap.String("source", "sarama")), zapcore.DebugLevel)
	if err != nil {
		log.Fatal("failed to create std logger for sarama", zap.Error(err))
	}
	sarama.Logger = saramaLogger

	// Sarama Config
	saramaConfig, err := kafka.NewSaramaConfig(&cfg.Kafka)
	if err != nil {
		log.Fatal("failed to create a valid sarama config", zap.Error(err))
	}

	// Sarama Client
	logger.Info("connecting to Kafka cluster")
	client, err := sarama.NewClient(cfg.Kafka.Brokers, saramaConfig)
	if err != nil {
		logger.Fatal("failed to create kafka client", zap.Error(err))
	}

	kafkaSvc := &kafka.Service{Client: client, Logger: logger, MetricsNamespace: cfg.MetricsNamespace}

	return &API{
		Cfg:      cfg,
		Logger:   logger,
		KafkaSvc: kafkaSvc,
		OwlSvc:   owl.NewService(kafkaSvc, logger),
		Hooks:    newDefaultHooks(),
	}
}

// Start the API server and block
func (api *API) Start() {
	api.KafkaSvc.RegisterMetrics()
	api.KafkaSvc.Start()

	// Server
	server := rest.NewServer(&api.Cfg.REST, api.Logger, api.routes())
	err := server.Start()
	if err != nil {
		api.Logger.Fatal("REST Server returned an error", zap.Error(err))
	}
}
