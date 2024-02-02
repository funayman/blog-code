package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cloud.google.com/go/storage"
	"github.com/ardanlabs/conf/v3"
	"github.com/funayman/logger"
	"go.uber.org/zap"

	"github.com/funayman/file-upload-to-bucket/web"
)

var (
	build = "dev"
)

func main() {
	log, err := logger.New("file-uploader")
	if err != nil {
		panic(err)
	}
	defer log.Sync()

	ctx := context.Background()
	if err := run(ctx, log); err != nil {
		log.Errorw("unable to run application", "error", err)
		os.Exit(2)
	}
}

func run(ctx context.Context, log *zap.SugaredLogger) error {
	config := struct {
		conf.Version
		Web struct {
			// ReadTimeout is critical here for uploading. This is the total amount
			// of time allotted for each request to read both the headers and the
			// body. As this copies directly from the uploaded file to the bucket
			// object, without local temporary storage, larger files might fail if the
			// connection pushing to GCS takes longer than one minute. Setting this
			// value to 0 will ensure that there is no limit.
			ReadTimeout  time.Duration `conf:"default:60s"`
			WriteTimeout time.Duration `conf:"default:10s"`
			IdleTimeout  time.Duration `conf:"default:120s"`
			// ShutdownTimeout provides the amount of time to keep a connection
			// running before cancelling all live connections on the server. Ideally
			// it should probably match ReadTimeout, as cancelling the io operation
			// during a copy will still leave a partial file in the bucket.
			ShutdownTimeout time.Duration `conf:"default:20s"`
			APIHost         string        `conf:"default:0.0.0.0:8000"`
		}
		Bucket string `conf:"required"`
	}{
		Version: conf.Version{
			Build: build,
		},
	}

	help, err := conf.Parse("", &config)
	switch {
	case errors.Is(err, conf.ErrHelpWanted):
		fmt.Println(help)
		return nil
	case err != nil:
		return fmt.Errorf("cannot parse config: %w", err)
	}

	out, err := conf.String(&config)
	if err != nil {
		return fmt.Errorf("generating config string: %w", err)
	}
	log.Infow("config loaded", "config", out)

	// SHUTDOWN SETUP ------------------------------------------------------------
	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, syscall.SIGINT, syscall.SIGTERM)

	// GCP BUCKET ----------------------------------------------------------------
	storageClient, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("cannot create GCS client: %w", err)
	}
	bkt := storageClient.Bucket(config.Bucket)

	// WEB SERVER ----------------------------------------------------------------
	serverErrorsCh := make(chan error, 1)

	mux := web.Mux(web.MuxConfig{
		Log:    log,
		Bucket: bkt,
	})

	srv := http.Server{
		Addr:         config.Web.APIHost,
		Handler:      mux,
		ReadTimeout:  config.Web.ReadTimeout,
		WriteTimeout: config.Web.WriteTimeout,
		IdleTimeout:  config.Web.IdleTimeout,
		ErrorLog:     logger.NewStdLogger(log),
	}

	go func() {
		log.Infow("starting web server", "host", srv.Addr)
		serverErrorsCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-serverErrorsCh:
		return fmt.Errorf("server returned error: %w", err)
	case sig := <-shutdownCh:
		log.Infow("shutdown initalized", "signal", sig)
		defer log.Infow("shutdown completed")

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), config.Web.ShutdownTimeout)
		defer shutdownCancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Errorw("could not shutdown web server properly", "error", err)
			srv.Close()
		}
		if err := storageClient.Close(); err != nil {
			log.Errorw("failure closing storageClient", "error", err)
		}
	}
	return nil
}
