package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	chatpkg "noto/internal/chat"
	"noto/internal/profile"
	"noto/internal/provider"
	"noto/internal/store"
	"noto/internal/web"
)

func webCmd() *cobra.Command {
	port := 8080
	command := &cobra.Command{
		Use:   "web",
		Short: "Start Noto's localhost web interface",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runWeb(cmd, port)
		},
	}
	command.Flags().IntVar(&port, "port", 8080, "localhost HTTP port (0 selects an available port)")
	return command
}

func listenWeb(ctx context.Context, port int) (net.Listener, error) {
	if port < 0 || port > 65535 {
		return nil, errors.New("web: port must be between 0 and 65535")
	}
	var listenerConfig net.ListenConfig
	return listenerConfig.Listen(ctx, "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
}

func runWeb(cmd *cobra.Command, port int) error {
	parentCtx := cmd.Context()
	if parentCtx == nil {
		parentCtx = context.Background()
	}
	ctx, stopSignals := signal.NotifyContext(parentCtx, os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	listener, err := listenWeb(ctx, port)
	if err != nil {
		return fmt.Errorf("web: listen on localhost: %w", err)
	}
	defer func() {
		if closeErr := listener.Close(); closeErr != nil && !errors.Is(closeErr, net.ErrClosed) {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "noto web: close listener: %v\n", closeErr)
		}
	}()

	profileSvc := profile.NewService(nil)
	startup, err := NewStartupFlow(profileSvc).Resolve(
		ctx,
		io.Discard,
		func() (string, error) { return "default", nil },
		func([]*store.Profile) (string, error) { return "", nil },
	)
	if err != nil {
		return err
	}
	activeProfile := startup.Profile
	profileDB, err := openProfileDB(activeProfile.Slug)
	if err != nil {
		return fmt.Errorf("web: open profile database: %w", err)
	}
	var session *chatpkg.Session
	skipDeferredCleanup := false
	defer func() {
		if skipDeferredCleanup {
			return
		}
		if session != nil {
			session.Close(context.Background())
		}
		if profileDB != nil {
			if closeErr := profileDB.Close(); closeErr != nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "noto web: close profile database: %v\n", closeErr)
			}
		}
	}()

	providerCfg, apiKey := loadProviderConfig(ctx, profileDB, activeProfile.ID)
	setupRequired := providerCfg == nil || apiKey == "" || providerCfg.EffectiveModel() == ""
	activeModel := ""
	if providerCfg != nil {
		activeModel = providerCfg.EffectiveModel()
	}
	if !setupRequired {
		systemPrompt, _ := loadSystemPrompt(ctx, profileDB, activeProfile)
		session, err = newConfiguredSession(ctx, profileDB, activeProfile, systemPrompt, providerCfg, apiKey, sessionCallbacks{})
		if err != nil {
			if errors.Is(err, errProviderSetupRequired) {
				setupRequired = true
				session = nil
			} else {
				return fmt.Errorf("web: start chat session: %w", err)
			}
		}
	}
	if session != nil {
		adapterCfg := provider.Config{ProviderType: "openai_compatible", Endpoint: providerCfg.Endpoint, APIKey: apiKey}
		extractorModel := providerCfg.ExtractorModel
		extractorFallback := extractorModel == ""
		if extractorFallback {
			extractorModel = activeModel
		}
		if models, listErr := provider.ListModels(ctx, adapterCfg); listErr == nil {
			applySessionModelMetadata(session, models, activeModel, extractorModel, extractorFallback)
		}
	}

	address := listener.Addr().(*net.TCPAddr)
	origin := fmt.Sprintf("http://127.0.0.1:%d", address.Port)
	var webSession web.ChatSession
	if session != nil {
		webSession = session
	}
	service := web.NewService(profileDB, activeProfile, webSession, activeModel, setupRequired)
	server := &http.Server{
		Handler:           web.NewHandler(service, origin),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	serveResult := make(chan error, 1)
	go func() { serveResult <- server.Serve(listener) }()
	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Server running on port %d: %s\n", address.Port, origin); err != nil {
		return fmt.Errorf("web: print server address: %w", err)
	}

	var runErr error
	select {
	case <-ctx.Done():
	case serveErr := <-serveResult:
		if !errors.Is(serveErr, http.ErrServerClosed) {
			runErr = fmt.Errorf("web: HTTP server: %w", serveErr)
		}
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()
	cleanupErr := cleanupWeb(shutdownCtx, server, service,
		func(context.Context) error {
			if session != nil {
				session.Close(context.Background())
				session = nil
			}
			return nil
		},
		func() error {
			if profileDB == nil {
				return nil
			}
			closeErr := profileDB.Close()
			profileDB = nil
			return closeErr
		},
	)
	if cleanupErr != nil {
		skipDeferredCleanup = true
		if runErr != nil {
			return errors.Join(runErr, cleanupErr)
		}
		return cleanupErr
	}
	if runErr != nil {
		return runErr
	}
	return nil
}

func cleanupWeb(ctx context.Context, server *http.Server, service *web.Service, closeSession func(context.Context) error, closeDB func() error) error {
	var stopErr, serverErr error
	if service != nil {
		stopErr = service.Stop(ctx)
	}
	if server != nil {
		serverErr = server.Shutdown(ctx)
	}
	if err := errors.Join(stopErr, serverErr); err != nil {
		return fmt.Errorf("web: shutdown did not drain active requests: %w", err)
	}
	if closeSession != nil {
		if err := closeSession(ctx); err != nil {
			return fmt.Errorf("web: close chat session: %w", err)
		}
	}
	if closeDB != nil {
		if err := closeDB(); err != nil {
			return fmt.Errorf("web: close profile database: %w", err)
		}
	}
	return nil
}
