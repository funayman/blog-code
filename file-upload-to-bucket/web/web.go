package web

import (
	"cloud.google.com/go/storage"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/funayman/file-upload-to-bucket/web/handler"
)

type MuxConfig struct {
	Log    *zap.SugaredLogger
	Bucket *storage.BucketHandle
}

func Mux(config MuxConfig) *chi.Mux {
	h := handler.New(config.Log, config.Bucket)

	mux := chi.NewMux()
	mux.With(middleware.AllowContentType("multipart/form-data")).Post("/upload", h.Upload)

	return mux
}
