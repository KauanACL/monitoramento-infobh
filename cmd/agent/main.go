package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/kardianos/service"

	"monitoramento-infobh/internal/agent"
)

type program struct {
	configPath string
	cancel     context.CancelFunc
	done       chan struct{}
	logger     *slog.Logger
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "install":
		if err := install(logger, os.Args[2:]); err != nil {
			logger.Error("install failed", "error", err)
			os.Exit(1)
		}
	case "reinstall":
		if err := reinstall(logger, os.Args[2:]); err != nil {
			logger.Error("reinstall failed", "error", err)
			os.Exit(1)
		}
	case "run":
		if err := runService(logger, os.Args[2:]); err != nil && err != context.Canceled {
			logger.Error("agent stopped", "error", err)
			os.Exit(1)
		}
	case "once":
		if err := runOnce(logger, os.Args[2:]); err != nil {
			logger.Error("agent once failed", "error", err)
			os.Exit(1)
		}
	case "start", "stop", "restart", "uninstall":
		if err := control(os.Args[1], logger, os.Args[2:]); err != nil {
			logger.Error("service command failed", "command", os.Args[1], "error", err)
			os.Exit(1)
		}
	case "config-path":
		fmt.Println(agent.DefaultConfigPath())
	default:
		usage()
		os.Exit(2)
	}
}

type installOptions struct {
	configPath string
	cfg        agent.Config
}

func install(logger *slog.Logger, args []string) error {
	opts, err := parseInstallOptions("install", args)
	if err != nil {
		return err
	}
	if err := requireServiceAdmin(); err != nil {
		return err
	}
	return installService(logger, opts)
}

func reinstall(logger *slog.Logger, args []string) error {
	opts, err := parseInstallOptions("reinstall", args)
	if err != nil {
		return err
	}
	if err := requireServiceAdmin(); err != nil {
		return err
	}

	svc, err := newService(opts.configPath, logger)
	if err != nil {
		return err
	}
	if err := service.Control(svc, "stop"); err != nil {
		if !ignorableReinstallControlError(err) {
			return err
		}
		logger.Info("service stop skipped", "reason", err)
	}
	if err := service.Control(svc, "uninstall"); err != nil {
		if !ignorableReinstallControlError(err) {
			return err
		}
		logger.Info("service uninstall skipped", "reason", err)
	}
	return installService(logger, opts)
}

func parseInstallOptions(command string, args []string) (installOptions, error) {
	fs := flag.NewFlagSet(command, flag.ExitOnError)
	configPath := fs.String("config", agent.DefaultConfigPath(), "caminho do arquivo de configuracao")
	serverURL := fs.String("server", "", "URL do servidor, ex: http://IP_DA_VM:8080")
	token := fs.String("token", "", "token gerado no dashboard")
	heartbeat := fs.Duration("heartbeat", 30*time.Second, "intervalo de heartbeat")
	metrics := fs.Duration("metrics", 60*time.Second, "intervalo de metricas")
	devices := fs.Duration("devices", 5*time.Minute, "intervalo de dispositivos")
	hardware := fs.Duration("hardware", 6*time.Hour, "intervalo de inventario de hardware")
	temperatures := fs.Duration("temperatures", 60*time.Second, "intervalo de temperaturas")
	commands := fs.Duration("commands", 15*time.Second, "intervalo para buscar comandos remotos")
	_ = fs.Parse(args)

	cfg := agent.DefaultConfig()
	cfg.ServerURL = *serverURL
	cfg.Token = *token
	cfg.HeartbeatEvery = *heartbeat
	cfg.MetricsEvery = *metrics
	cfg.DevicesEvery = *devices
	cfg.HardwareEvery = *hardware
	cfg.TemperaturesEvery = *temperatures
	cfg.CommandsEvery = *commands
	if err := cfg.Validate(); err != nil {
		return installOptions{}, err
	}
	return installOptions{configPath: *configPath, cfg: cfg}, nil
}

func installService(logger *slog.Logger, opts installOptions) error {
	if err := agent.SaveConfig(opts.configPath, opts.cfg); err != nil {
		return err
	}

	svc, err := newService(opts.configPath, logger)
	if err != nil {
		return err
	}
	if err := service.Control(svc, "install"); err != nil {
		return err
	}
	logger.Info("service installed", "config", opts.configPath)
	return service.Control(svc, "start")
}

func ignorableReinstallControlError(err error) bool {
	if err == nil {
		return true
	}
	msg := strings.ToLower(err.Error())
	parts := []string{
		"does not exist",
		"not exist",
		"not installed",
		"has not been installed",
		"has not been started",
		"não existe",
		"nao existe",
		"não foi iniciado",
		"nao foi iniciado",
	}
	for _, part := range parts {
		if strings.Contains(msg, part) {
			return true
		}
	}
	return false
}

func control(command string, logger *slog.Logger, args []string) error {
	fs := flag.NewFlagSet(command, flag.ExitOnError)
	configPath := fs.String("config", agent.DefaultConfigPath(), "caminho do arquivo de configuracao")
	_ = fs.Parse(args)

	svc, err := newService(*configPath, logger)
	if err != nil {
		return err
	}
	return service.Control(svc, command)
}

func runService(logger *slog.Logger, args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	configPath := fs.String("config", agent.DefaultConfigPath(), "caminho do arquivo de configuracao")
	_ = fs.Parse(args)

	svc, err := newService(*configPath, logger)
	if err != nil {
		return err
	}
	return svc.Run()
}

func runOnce(logger *slog.Logger, args []string) error {
	fs := flag.NewFlagSet("once", flag.ExitOnError)
	configPath := fs.String("config", agent.DefaultConfigPath(), "caminho do arquivo de configuracao")
	serverURL := fs.String("server", "", "URL do servidor")
	token := fs.String("token", "", "token gerado no dashboard")
	_ = fs.Parse(args)

	cfg, err := agent.LoadConfig(*configPath)
	if err != nil {
		cfg = agent.DefaultConfig()
		cfg.ServerURL = *serverURL
		cfg.Token = *token
	}
	if *serverURL != "" {
		cfg.ServerURL = *serverURL
	}
	if *token != "" {
		cfg.Token = *token
	}
	if err := cfg.Validate(); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return agent.NewRunner(cfg, logger).Once(ctx)
}

func newService(configPath string, logger *slog.Logger) (service.Service, error) {
	cfg := &service.Config{
		Name:        agent.DefaultServiceName,
		DisplayName: "InfoBH Monitor Agent",
		Description: "Coleta metricas, hardware, temperaturas, dispositivos e comandos para o Monitoramento InfoBH.",
		Arguments:   []string{"run", "-config", configPath},
	}
	return service.New(&program{configPath: configPath, logger: logger}, cfg)
}

func (p *program) Start(s service.Service) error {
	cfg, err := agent.LoadConfig(p.configPath)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	p.done = make(chan struct{})
	go func() {
		defer close(p.done)
		if err := agent.NewRunner(cfg, p.logger).Run(ctx); err != nil && err != context.Canceled {
			p.logger.Warn("runner stopped", "error", err)
		}
	}()
	return nil
}

func (p *program) Stop(s service.Service) error {
	if p.cancel != nil {
		p.cancel()
	}
	if p.done != nil {
		select {
		case <-p.done:
		case <-time.After(15 * time.Second):
		}
	}
	return nil
}

func usage() {
	fmt.Println(`InfoBH Monitor Agent

Comandos:
  install -server http://IP_DA_VM:8080 -token TOKEN     grava config, instala e inicia o servico
  reinstall -server http://IP_DA_VM:8080 -token TOKEN   para, remove, instala e inicia o servico
  run [-config caminho]                                executa como servico/console
  once [-config caminho]                               envia uma coleta e encerra
  start|stop|restart|uninstall                         controla o servico
  config-path                                          mostra o caminho padrao da configuracao`)
}
