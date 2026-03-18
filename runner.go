package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/paczulapiotr/quiz-lab/lights/dmx"
	"github.com/paczulapiotr/quiz-lab/lights/internal/effects"
	"github.com/paczulapiotr/quiz-lab/lights/internal/effects/devices"
	"github.com/paczulapiotr/quiz-lab/lights/internal/models"
)

// Config holds the runtime configuration resolved from CLI flags.
type Config struct {
	RabbitMQURL string
	Queue       string
	Interval    time.Duration
}

// RunController opens the DMX adapter, registers fixtures, connects to RabbitMQ,
// and dispatches incoming messages to the effect manager until a shutdown signal
// or the delivery channel closes.
func RunController(cfg Config, logger *log.Logger) error {
	ctrl, err := openDMX(cfg, logger)
	if err != nil {
		return err
	}
	defer ctrl.Close()

	mgr := buildManager(ctrl, logger)

	deliveries, conn, ch, err := connectAMQP(cfg, logger)
	if err != nil {
		return err
	}
	defer conn.Close()
	defer ch.Close()

	return consumeLoop(deliveries, mgr, logger)
}

// openDMX opens the USB DMX adapter and starts auto-send.
func openDMX(cfg Config, logger *log.Logger) (*dmx.USBController, error) {
	logger.Println("[dmx] opening USB DMX adapter")
	ctrl, err := dmx.OpenUSB()
	if err != nil {
		return nil, fmt.Errorf("open USB DMX adapter: %w", err)
	}
	ctrl.StartAutoSend(cfg.Interval)
	logger.Printf("[dmx] auto-send interval: %s", cfg.Interval)
	return ctrl, nil
}

// buildManager creates the effect manager and registers all supported fixtures.
func buildManager(device effects.DMXDevice, logger *log.Logger) *effects.Manager {
	mgr := effects.NewManager(device, logger)
	mgr.RegisterFixture("pst10", devices.PST10Fixture())
	mgr.RegisterFixture("b40", devices.B40Fixture())
	logger.Println("[dmx] registered fixtures: pst10, b40")
	return mgr
}

// connectAMQP dials RabbitMQ, declares the queue, and starts a consumer.
func connectAMQP(cfg Config, logger *log.Logger) (<-chan amqp.Delivery, *amqp.Connection, *amqp.Channel, error) {
	logger.Printf("[amqp] connecting to %s", cfg.RabbitMQURL)
	conn, err := amqp.Dial(cfg.RabbitMQURL)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, nil, nil, fmt.Errorf("open AMQP channel: %w", err)
	}

	q, err := ch.QueueDeclare(cfg.Queue, true, false, false, false, nil)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, nil, nil, fmt.Errorf("declare queue %q: %w", cfg.Queue, err)
	}

	deliveries, err := ch.Consume(q.Name, "dmxcontroller", false, false, false, false, nil)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, nil, nil, fmt.Errorf("start consuming from %q: %w", cfg.Queue, err)
	}

	logger.Printf("[amqp] consuming from queue %q", cfg.Queue)
	return deliveries, conn, ch, nil
}

// consumeLoop dispatches deliveries to the effect manager until shutdown.
func consumeLoop(deliveries <-chan amqp.Delivery, mgr *effects.Manager, logger *log.Logger) error {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-quit:
			logger.Println("[dmx] shutting down – stopping all effects")
			mgr.StopAll()
			return nil

		case d, ok := <-deliveries:
			if !ok {
				logger.Println("[amqp] delivery channel closed")
				mgr.StopAll()
				return nil
			}

			var msg models.DMXMessage
			if err := json.Unmarshal(d.Body, &msg); err != nil {
				logger.Printf("[amqp] invalid message: %v – body: %s", err, d.Body)
				_ = d.Nack(false, false)
				continue
			}

			logger.Printf("[amqp] received  addr=%-3d type=%-6s action=%s",
				msg.StartAddress, msg.LightType, msg.Action)

			if err := mgr.Apply(msg.StartAddress, msg.LightType, msg.Action); err != nil {
				logger.Printf("[dmx]  apply error: %v", err)
				_ = d.Nack(false, false)
				continue
			}

			_ = d.Ack(false)
		}
	}
}
