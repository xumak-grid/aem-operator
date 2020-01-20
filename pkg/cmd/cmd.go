package cmd

import (
	"os"
	"os/signal"

	"github.com/xumak-grid/aem-operator/pkg/operator"
	"go.uber.org/zap"
)

// RunOperator runs the operator controller.
func RunOperator(cfg string, logger *zap.Logger) error {
	signals := make(chan os.Signal)
	stop := make(chan struct{})
	signal.Notify(signals, os.Interrupt, os.Kill)
	operator, err := operator.NewAEMController(cfg, logger)
	if err != nil {
		return err
	}
	go operator.Run(stop)
	<-signals
	close(stop)
	logger.Info("Shutting down AEM Operator")
	return nil
}
