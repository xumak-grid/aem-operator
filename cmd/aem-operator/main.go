package main

import (
	"flag"
	"os"

	"github.com/xumak-grid/aem-operator/pkg/cmd"
	"github.com/xumak-grid/aem-operator/version"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	kubeConfig := flag.String("kubeconfig", "", "path to kubeconfig file, required for out of cluster e.g: ~/.kube/config")
	flag.Parse()

	logger, _ := getLogger()
	checkEnvVar(logger)
	logger.Info("Initializing AEM Operator")
	err := cmd.RunOperator(*kubeConfig, logger)
	if err != nil {
		logger.Error("Error running operator", zap.Error(err))
	}
}

func getLogger() (*zap.Logger, error) {
	config := zap.NewProductionConfig()
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	return config.Build(zap.Fields(zap.String("operator_version", version.Version)))
}

// checkEnvVar checks critical environment variables and exits if one is not present
func checkEnvVar(log *zap.Logger) {
	if os.Getenv("VAULT_ADDR") == "" {
		log.Fatal("VAULT_ADDR is not set and is required")
	}
	if os.Getenv("VAULT_TOKEN") == "" {
		log.Fatal("VAULT_TOKEN is not set and is required")
	}
	if os.Getenv("GRID_EXTERNAL_DOMAIN") == "" {
		log.Fatal("GRID_EXTERNAL_DOMAIN is not set and is required")
	}
}
