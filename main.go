package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/spf13/cobra"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "dmxcontroller",
	Short: "DMX lighting controller driven by RabbitMQ messages",
	Long: `dmxcontroller connects to a RabbitMQ queue and applies DMX effects to
fixtures based on incoming JSON messages. It uses a USB DMX adapter
and can drive multiple fixtures simultaneously.

Message format:
  {"start_address": 11, "light_type": "pst10", "action": "green"}

Supported light types: pst10, b40
Supported actions:     red, green, blue, white, warm, off, rainbow  (b40 also: strobe)`,
	// SilenceUsage prevents the full help text from printing on every runtime
	// error (e.g. a lost RabbitMQ connection). Help is shown explicitly below
	// only when configuration values are invalid.
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, _ []string) error {
		f := cmd.Flags()
		rabbitmqURL, _ := f.GetString("rabbitmq-url")
		queue, _ := f.GetString("queue")
		interval, _ := f.GetDuration("interval")

		if err := validateConfig(rabbitmqURL, queue, interval); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			fmt.Fprintln(os.Stderr)
			_ = cmd.Help()
			return errors.New("invalid configuration")
		}

		cfg := Config{
			RabbitMQURL: rabbitmqURL,
			Queue:       queue,
			Interval:    interval,
		}
		logger := log.New(os.Stdout, "", log.LstdFlags|log.Lmicroseconds)
		return RunController(cfg, logger)
	},
}

func init() {
	f := rootCmd.Flags()
	f.String("rabbitmq-url", "amqp://guest:guest@localhost:5672/", "RabbitMQ connection URL")
	f.String("queue", "dmx", "RabbitMQ queue name to consume from")
	f.Duration("interval", 50*time.Millisecond, "DMX auto-send interval")

}

func validateConfig(rabbitmqURL, queue string, interval time.Duration) error {
	if rabbitmqURL == "" {
		return errors.New("--rabbitmq-url must not be empty")
	}
	if queue == "" {
		return errors.New("--queue must not be empty")
	}
	if interval <= 0 {
		return fmt.Errorf("--interval must be positive, got %s", interval)
	}
	return nil
}
