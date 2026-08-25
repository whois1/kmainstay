package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"kmainstay/internal/database"
)

const (
	maximumImageBytes    = 10 << 20
	maximumImagePixels   = 16_000_000
	multipartMemoryBytes = 1 << 20
)

type attachment struct {
	ID               string `json:"id"`
	MessageID        string `json:"-"`
	StorageKey       string `json:"-"`
	MediaType        string `json:"media_type"`
	ByteSize         int64  `json:"byte_size"`
	Width            int    `json:"width"`
	Height           int    `json:"height"`
	OriginalFilename string `json:"original_filename"`
	SHA256           string `json:"-"`
	CreatedAt        string `json:"created_at"`
	ContentURL       string `json:"content_url"`
}

type pendingAttachment struct {
	attachment
	Bytes []byte
}

type messageInput struct {
	Body             string
	ClientID         string
	ReplyToMessageID string
	Attachment       *pendingAttachment
}

func (s *server) decodeMessageInput(w http.ResponseWriter, r *http.Request) (messageInput, bool) {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		var input struct {
			Body             string `json:"body"`
			ClientID         string `json:"client_id"`
			ReplyToMessageID string `json:"reply_to_message_id"`
		}
		if !decode(w, r, &input) {
			return messageInput{}, false
		}
		return messageInput{Body: input.Body, ClientID: input.ClientID, ReplyToMessageID: input.ReplyToMessageID}, true
	}
	select {
	case s.imageUploadSlots <- struct{}{}:
		defer func() { <-s.imageUploadSlots }()
	default:
		writeError(w, http.StatusServiceUnavailable, "image upload capacity exceeded")
		return messageInput{}, false
	}
	responseController := http.NewResponseController(w)
	if err := responseController.SetReadDeadline(time.Now().Add(60 * time.Second)); err == nil {
		defer responseController.SetReadDeadline(time.Time{})
	}
	r.Body = http.MaxBytesReader(w, r.Body, maximumImageBytes+(64<<10))
	if err := r.ParseMultipartForm(multipartMemoryBytes); err != nil {
		writeError(w, http.StatusBadRequest, "invalid image upload")
		return messageInput{}, false
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	input := messageInput{Body: r.FormValue("body"), ClientID: r.FormValue("client_id"), ReplyToMessageID: r.FormValue("reply_to_message_id")}
	files := r.MultipartForm.File["image"]
	if len(files) == 0 {
		return input, true
	}
	if len(files) != 1 || len(r.MultipartForm.File) != 1 {
		writeError(w, http.StatusBadRequest, "one image is allowed")
		return messageInput{}, false
	}
	declaredMediaType, _, err := mime.ParseMediaType(files[0].Header.Get("Content-Type"))
	if err != nil || (declaredMediaType != "image/jpeg" && declaredMediaType != "image/png") {
		writeError(w, http.StatusBadRequest, "image must declare JPEG or PNG content")
		return messageInput{}, false
	}
	file, err := files[0].Open()
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid image upload")
		return messageInput{}, false
	}
	data, err := io.ReadAll(io.LimitReader(file, maximumImageBytes+1))
	file.Close()
	if err != nil || len(data) == 0 || len(data) > maximumImageBytes {
		writeError(w, http.StatusBadRequest, "invalid image upload")
		return messageInput{}, false
	}
	mediaType := http.DetectContentType(data)
	if mediaType != declaredMediaType {
		writeError(w, http.StatusBadRequest, "image must be JPEG or PNG")
		return messageInput{}, false
	}
	configuration, decodedType, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || "image/"+decodedType != mediaType || configuration.Width <= 0 || configuration.Height <= 0 || configuration.Width > 10_000 || configuration.Height > 10_000 || int64(configuration.Width)*int64(configuration.Height) > maximumImagePixels {
		writeError(w, http.StatusBadRequest, "invalid image dimensions")
		return messageInput{}, false
	}
	decodedImage, fullyDecodedType, err := image.Decode(bytes.NewReader(data))
	if err != nil || fullyDecodedType != decodedType || decodedImage.Bounds().Dx() != configuration.Width || decodedImage.Bounds().Dy() != configuration.Height {
		writeError(w, http.StatusBadRequest, "invalid image data")
		return messageInput{}, false
	}
	filename := filepath.Base(strings.TrimSpace(files[0].Filename))
	filename = strings.ToValidUTF8(filename, "�")
	if filename == "." || filename == "" {
		filename = "image"
	}
	if len(filename) > 255 {
		filename = filename[:255]
		for !utf8.ValidString(filename) {
			filename = filename[:len(filename)-1]
		}
	}
	digest := sha256.Sum256(data)
	item := attachment{
		ID:               database.NewID("att"),
		StorageKey:       database.NewID("obj"),
		MediaType:        mediaType,
		ByteSize:         int64(len(data)),
		Width:            configuration.Width,
		Height:           configuration.Height,
		OriginalFilename: filename,
		SHA256:           hex.EncodeToString(digest[:]),
	}
	item.ContentURL = "/api/attachments/" + item.ID + "/content"
	input.Attachment = &pendingAttachment{attachment: item, Bytes: data}
	return input, true
}

func (s *server) messageAttachments(ctx context.Context, messageID string) ([]attachment, error) {
	return queryMessageAttachments(ctx, s.db, messageID)
}

func queryMessageAttachments(ctx context.Context, queryable queryer, messageID string) ([]attachment, error) {
	rows, err := queryable.QueryContext(ctx, `SELECT id,message_id,storage_key,media_type,byte_size,width,height,original_filename,sha256,created_at FROM attachments WHERE message_id=? ORDER BY created_at,id`, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []attachment{}
	for rows.Next() {
		var item attachment
		if err := rows.Scan(&item.ID, &item.MessageID, &item.StorageKey, &item.MediaType, &item.ByteSize, &item.Width, &item.Height, &item.OriginalFilename, &item.SHA256, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.ContentURL = "/api/attachments/" + item.ID + "/content"
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *server) attachmentContent(w http.ResponseWriter, r *http.Request) {
	if s.attachments == nil {
		returnServerError(w)
		return
	}
	var item attachment
	err := s.db.QueryRowContext(r.Context(), `SELECT attachment.id,attachment.message_id,attachment.storage_key,attachment.media_type,attachment.byte_size,attachment.width,attachment.height,attachment.original_filename,attachment.sha256,attachment.created_at
		FROM attachments attachment
		JOIN messages message ON message.id=attachment.message_id
		JOIN conversations conversation ON conversation.id=message.conversation_id
		JOIN organisation_memberships membership ON membership.organisation_id=conversation.organisation_id AND membership.user_id=?
		WHERE attachment.id=? AND (conversation.visibility='organisation' OR EXISTS(SELECT 1 FROM conversation_members participant WHERE participant.conversation_id=conversation.id AND participant.user_id=?))`, current(r).ID, r.PathValue("attachment"), current(r).ID).Scan(&item.ID, &item.MessageID, &item.StorageKey, &item.MediaType, &item.ByteSize, &item.Width, &item.Height, &item.OriginalFilename, &item.SHA256, &item.CreatedAt)
	if errors.Is(err, context.Canceled) {
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		returnServerError(w)
		return
	}
	reader, err := s.attachments.Open(item.StorageKey)
	if err != nil {
		returnServerError(w)
		return
	}
	defer reader.Close()
	w.Header().Set("Content-Type", item.MediaType)
	w.Header().Set("Content-Length", strconv.FormatInt(item.ByteSize, 10))
	w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": item.OriginalFilename}))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("ETag", `"`+item.SHA256+`"`)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if _, err := io.Copy(w, reader); err != nil {
		return
	}
}
