package handler

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"cloud.google.com/go/storage"
	"go.uber.org/zap"
)

type Handler struct {
	bucket *storage.BucketHandle
	log    *zap.SugaredLogger
}

func New(log *zap.SugaredLogger, bucket *storage.BucketHandle) *Handler {
	return &Handler{
		log:    log,
		bucket: bucket,
	}
}

func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	u, err := h.upload(r)
	if err != nil {
		h.log.Errorw("error during upload", "err", err)
		switch {
		case isValidationError(err):
			http.Error(w, err.Error(), http.StatusBadRequest)
		default:
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}
	h.log.Infow("file uploaded", "url", u)
	fmt.Fprintln(w, u)
}

func (h *Handler) upload(r *http.Request) (string, error) {
	mpr, err := r.MultipartReader()
	if err != nil {
		return "", fmt.Errorf("r.MultipartReader failure: %w", err)
	}

	// HANDLE FILE RENAME --------------------------------------------------------
	part, err := mpr.NextPart()
	switch {
	case err != nil:
		return "", fmt.Errorf("mpr.NextPart failure on first read: %w", err)
	case part.FormName() != "name":
		return "", validationError("expected name first")
	}

	buf := bytes.NewBuffer(nil)
	if _, err := io.Copy(buf, part); err != nil && err != io.EOF {
		return "", fmt.Errorf("io.Copy failure for name: %w", err)
	}
	uploadFilename := buf.String()

	// HANDLE FILE UPLOAD --------------------------------------------------------
	part, err = mpr.NextPart()
	switch {
	case err != nil:
		return "", fmt.Errorf("mpr.NextPart failure on second read: %w", err)
	case part.FormName() != "myfile":
		return "", validationError("file required")
	}

	if uploadFilename == "" {
		uploadFilename = part.FileName()
	}

	// WRITE TO BUCKET -----------------------------------------------------------

	// create new/update bucket object
	obj := h.bucket.Object(uploadFilename)

	// copy data to object
	sw := obj.NewWriter(r.Context())
	if _, err := io.Copy(sw, part); err != nil {
		return "", fmt.Errorf("io.Copy failure for file: %w", err)
	}

	// close writer; attrs will be nil if not closed; cannot defer
	if err := sw.Close(); err != nil {
		return "", fmt.Errorf("unable to close file after writing object: %w", err)
	}

	// get URL of newly created obj
	attrs, err := obj.Attrs(r.Context())
	if err != nil {
		return "", fmt.Errorf("cannot generate attrs for object: %w", err)
	}

	return attrs.MediaLink, nil
}
