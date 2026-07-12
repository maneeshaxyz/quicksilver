package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"quicksilver/server/internal/http/middleware"
	"quicksilver/server/internal/httpx"
	himap "quicksilver/server/internal/imap"
	"quicksilver/server/internal/mail"
	"quicksilver/server/internal/session"
	"quicksilver/server/internal/smtp"
)

// Messages serves single-message read/write endpoints.
type Messages struct {
	Sessions *session.Store
	Sealer   *session.Sealer
	Sender   *smtp.Sender
	Logger   *slog.Logger
}

// Get returns the full message at the given UID.
func (h *Messages) Get(w http.ResponseWriter, r *http.Request) {
	sess, ok := middleware.SessionFrom(r.Context())
	if !ok {
		httpx.WriteError(w, r, h.Logger, httpx.NewAPIError(http.StatusUnauthorized, httpx.CodeUnauthorized, "no session", nil))
		return
	}
	mailbox, err := url.PathUnescape(chi.URLParam(r, "mailbox"))
	if err != nil || mailbox == "" {
		httpx.WriteError(w, r, h.Logger, httpx.NewAPIError(http.StatusBadRequest, httpx.CodeBadRequest, "invalid mailbox name", err))
		return
	}
	uid, err := strconv.ParseUint(chi.URLParam(r, "uid"), 10, 32)
	if err != nil {
		httpx.WriteError(w, r, h.Logger, httpx.NewAPIError(http.StatusBadRequest, httpx.CodeBadRequest, "invalid uid", err))
		return
	}
	c, err := h.Sessions.IMAPFor(r.Context(), sess)
	if err != nil {
		httpx.WriteError(w, r, h.Logger, httpx.NewAPIError(http.StatusBadGateway, httpx.CodeUpstreamFailed, "imap connect", err))
		return
	}
	msg, err := c.GetMessage(r.Context(), mailbox, uint32(uid))
	if err != nil {
		if errors.Is(err, himap.ErrNotFound) {
			httpx.WriteError(w, r, h.Logger, httpx.NewAPIError(http.StatusNotFound, httpx.CodeNotFound, "message not found", nil))
			return
		}
		httpx.WriteError(w, r, h.Logger, httpx.NewAPIError(http.StatusBadGateway, httpx.CodeUpstreamFailed, "fetch message", err))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, msg)
}

// GetAttachment streams a single attachment's bytes. The attachment is
// identified by the ID handed to the client in the message's attachment list
// ("att-1", "att-2", ...). Viewable, low-risk types (images, PDF, plain text)
// are served inline so the browser can render them in a new tab; everything
// else is forced to a download. text/html is never served inline, and the
// global nosniff header prevents a mislabelled type from being sniffed into
// active content.
func (h *Messages) GetAttachment(w http.ResponseWriter, r *http.Request) {
	sess, ok := middleware.SessionFrom(r.Context())
	if !ok {
		httpx.WriteError(w, r, h.Logger, httpx.NewAPIError(http.StatusUnauthorized, httpx.CodeUnauthorized, "no session", nil))
		return
	}
	mailbox, err := url.PathUnescape(chi.URLParam(r, "mailbox"))
	if err != nil || mailbox == "" {
		httpx.WriteError(w, r, h.Logger, httpx.NewAPIError(http.StatusBadRequest, httpx.CodeBadRequest, "invalid mailbox name", err))
		return
	}
	uid, err := strconv.ParseUint(chi.URLParam(r, "uid"), 10, 32)
	if err != nil {
		httpx.WriteError(w, r, h.Logger, httpx.NewAPIError(http.StatusBadRequest, httpx.CodeBadRequest, "invalid uid", err))
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.WriteError(w, r, h.Logger, httpx.NewAPIError(http.StatusBadRequest, httpx.CodeBadRequest, "invalid attachment id", nil))
		return
	}
	c, err := h.Sessions.IMAPFor(r.Context(), sess)
	if err != nil {
		httpx.WriteError(w, r, h.Logger, httpx.NewAPIError(http.StatusBadGateway, httpx.CodeUpstreamFailed, "imap connect", err))
		return
	}
	meta, data, err := c.GetAttachment(r.Context(), mailbox, uint32(uid), id)
	if err != nil {
		if errors.Is(err, himap.ErrNotFound) {
			httpx.WriteError(w, r, h.Logger, httpx.NewAPIError(http.StatusNotFound, httpx.CodeNotFound, "attachment not found", nil))
			return
		}
		httpx.WriteError(w, r, h.Logger, httpx.NewAPIError(http.StatusBadGateway, httpx.CodeUpstreamFailed, "fetch attachment", err))
		return
	}

	ct := meta.MIMEType
	if ct == "" {
		ct = "application/octet-stream"
	}
	disposition := "attachment"
	if r.URL.Query().Get("download") != "1" && inlineViewable(ct) {
		disposition = "inline"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("Content-Disposition", fmt.Sprintf("%s; filename=%q", disposition, sanitizeFilename(meta.Filename)))
	// Attachments are immutable for a given UID; let the browser cache briefly.
	w.Header().Set("Cache-Control", "private, max-age=300")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// inlineViewable reports whether a MIME type is safe to render inline in the
// browser. Deliberately excludes text/html (script-injection risk) and any
// active content — those download instead.
func inlineViewable(mimeType string) bool {
	mt := strings.ToLower(strings.TrimSpace(mimeType))
	if i := strings.IndexByte(mt, ';'); i >= 0 {
		mt = strings.TrimSpace(mt[:i])
	}
	if strings.HasPrefix(mt, "image/") {
		return true
	}
	switch mt {
	case "application/pdf", "text/plain":
		return true
	}
	return false
}

// sanitizeFilename strips path separators and control characters so a hostile
// upstream filename can't break out of the Content-Disposition header or
// suggest a traversal path to the browser. Falls back to a generic name.
func sanitizeFilename(name string) string {
	name = strings.Map(func(rr rune) rune {
		switch {
		case rr < 0x20, rr == '"', rr == '\\', rr == '/', rr == 0x7f:
			return -1
		default:
			return rr
		}
	}, name)
	name = strings.TrimSpace(name)
	if name == "" {
		return "attachment"
	}
	return name
}

type flagsRequest struct {
	Flags []string `json:"flags"`
	Add   bool     `json:"add"`
}

// SetFlags toggles flags on a message.
func (h *Messages) SetFlags(w http.ResponseWriter, r *http.Request) {
	sess, ok := middleware.SessionFrom(r.Context())
	if !ok {
		httpx.WriteError(w, r, h.Logger, httpx.NewAPIError(http.StatusUnauthorized, httpx.CodeUnauthorized, "no session", nil))
		return
	}
	mailbox, err := url.PathUnescape(chi.URLParam(r, "mailbox"))
	if err != nil || mailbox == "" {
		httpx.WriteError(w, r, h.Logger, httpx.NewAPIError(http.StatusBadRequest, httpx.CodeBadRequest, "invalid mailbox name", err))
		return
	}
	uid, err := strconv.ParseUint(chi.URLParam(r, "uid"), 10, 32)
	if err != nil {
		httpx.WriteError(w, r, h.Logger, httpx.NewAPIError(http.StatusBadRequest, httpx.CodeBadRequest, "invalid uid", err))
		return
	}
	var req flagsRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8*1024)).Decode(&req); err != nil {
		httpx.WriteError(w, r, h.Logger, httpx.NewAPIError(http.StatusBadRequest, httpx.CodeBadRequest, "invalid json", err))
		return
	}
	if len(req.Flags) == 0 {
		httpx.WriteError(w, r, h.Logger, httpx.NewAPIError(http.StatusBadRequest, httpx.CodeBadRequest, "flags required", nil))
		return
	}
	c, err := h.Sessions.IMAPFor(r.Context(), sess)
	if err != nil {
		httpx.WriteError(w, r, h.Logger, httpx.NewAPIError(http.StatusBadGateway, httpx.CodeUpstreamFailed, "imap connect", err))
		return
	}
	if err := c.SetFlags(r.Context(), mailbox, uint32(uid), req.Flags, req.Add); err != nil {
		httpx.WriteError(w, r, h.Logger, httpx.NewAPIError(http.StatusBadGateway, httpx.CodeUpstreamFailed, "set flags", err))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type deleteRequest struct {
	Trash string `json:"trash"`
	// Permanent skips the trash move and expunges the message in place —
	// the "delete from Trash forever" path.
	Permanent bool `json:"permanent"`
}

// Delete moves the message to the Trash mailbox, or — with {"permanent":true}
// — expunges it in place.
//
// The destination defaults to "Trash"; clients may override via body to support
// providers using a localised or non-standard trash folder name.
func (h *Messages) Delete(w http.ResponseWriter, r *http.Request) {
	sess, ok := middleware.SessionFrom(r.Context())
	if !ok {
		httpx.WriteError(w, r, h.Logger, httpx.NewAPIError(http.StatusUnauthorized, httpx.CodeUnauthorized, "no session", nil))
		return
	}
	mailbox, err := url.PathUnescape(chi.URLParam(r, "mailbox"))
	if err != nil || mailbox == "" {
		httpx.WriteError(w, r, h.Logger, httpx.NewAPIError(http.StatusBadRequest, httpx.CodeBadRequest, "invalid mailbox name", err))
		return
	}
	uid, err := strconv.ParseUint(chi.URLParam(r, "uid"), 10, 32)
	if err != nil {
		httpx.WriteError(w, r, h.Logger, httpx.NewAPIError(http.StatusBadRequest, httpx.CodeBadRequest, "invalid uid", err))
		return
	}
	dest := "Trash"
	permanent := false
	if r.ContentLength > 0 {
		var req deleteRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4*1024)).Decode(&req); err == nil {
			if req.Trash != "" {
				dest = req.Trash
			}
			permanent = req.Permanent
		}
	}
	c, err := h.Sessions.IMAPFor(r.Context(), sess)
	if err != nil {
		httpx.WriteError(w, r, h.Logger, httpx.NewAPIError(http.StatusBadGateway, httpx.CodeUpstreamFailed, "imap connect", err))
		return
	}
	if permanent {
		if err := c.Delete(r.Context(), mailbox, uint32(uid)); err != nil {
			httpx.WriteError(w, r, h.Logger, httpx.NewAPIError(http.StatusBadGateway, httpx.CodeUpstreamFailed, "permanent delete", err))
			return
		}
	} else if err := c.Move(r.Context(), mailbox, uint32(uid), dest); err != nil {
		httpx.WriteError(w, r, h.Logger, httpx.NewAPIError(http.StatusBadGateway, httpx.CodeUpstreamFailed, "move to trash", err))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Find returns the UID of the message carrying the given Message-ID header in
// a mailbox. Clients use it to relocate a message after a cross-mailbox move
// (e.g. to restore or permanently delete a message just moved to Trash).
func (h *Messages) Find(w http.ResponseWriter, r *http.Request) {
	sess, ok := middleware.SessionFrom(r.Context())
	if !ok {
		httpx.WriteError(w, r, h.Logger, httpx.NewAPIError(http.StatusUnauthorized, httpx.CodeUnauthorized, "no session", nil))
		return
	}
	mailbox, err := url.PathUnescape(chi.URLParam(r, "mailbox"))
	if err != nil || mailbox == "" {
		httpx.WriteError(w, r, h.Logger, httpx.NewAPIError(http.StatusBadRequest, httpx.CodeBadRequest, "invalid mailbox name", err))
		return
	}
	messageID := r.URL.Query().Get("message_id")
	if messageID == "" {
		httpx.WriteError(w, r, h.Logger, httpx.NewAPIError(http.StatusBadRequest, httpx.CodeBadRequest, "missing message_id", nil))
		return
	}
	c, err := h.Sessions.IMAPFor(r.Context(), sess)
	if err != nil {
		httpx.WriteError(w, r, h.Logger, httpx.NewAPIError(http.StatusBadGateway, httpx.CodeUpstreamFailed, "imap connect", err))
		return
	}
	uid, err := c.FindByMessageID(r.Context(), mailbox, messageID)
	if err != nil {
		if errors.Is(err, himap.ErrNotFound) {
			httpx.WriteError(w, r, h.Logger, httpx.NewAPIError(http.StatusNotFound, httpx.CodeNotFound, "message not found", nil))
			return
		}
		httpx.WriteError(w, r, h.Logger, httpx.NewAPIError(http.StatusBadGateway, httpx.CodeUpstreamFailed, "search message", err))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]uint32{"uid": uid})
}

type sendRequest struct {
	To         []mail.Address `json:"to"`
	Cc         []mail.Address `json:"cc,omitempty"`
	Bcc        []mail.Address `json:"bcc,omitempty"`
	Subject    string         `json:"subject"`
	BodyText   string         `json:"body_text,omitempty"`
	BodyHTML   string         `json:"body_html,omitempty"`
	InReplyTo  string         `json:"in_reply_to,omitempty"`
	References []string       `json:"references,omitempty"`
	// Attachments are out-of-band: clients POST a multipart upload separately
	// or this endpoint can be switched to multipart/form-data later.
}

// Send delivers a message via SMTP using the session's stored credentials.
func (h *Messages) Send(w http.ResponseWriter, r *http.Request) {
	sess, ok := middleware.SessionFrom(r.Context())
	if !ok {
		httpx.WriteError(w, r, h.Logger, httpx.NewAPIError(http.StatusUnauthorized, httpx.CodeUnauthorized, "no session", nil))
		return
	}
	var req sendRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2*1024*1024)).Decode(&req); err != nil {
		httpx.WriteError(w, r, h.Logger, httpx.NewAPIError(http.StatusBadRequest, httpx.CodeBadRequest, "invalid json", err))
		return
	}
	if len(req.To) == 0 || req.Subject == "" || (req.BodyText == "" && req.BodyHTML == "") {
		httpx.WriteError(w, r, h.Logger, httpx.NewAPIError(http.StatusBadRequest, httpx.CodeBadRequest, "to, subject, and a body are required", nil))
		return
	}
	creds, err := sess.Credentials(h.Sealer)
	if err != nil {
		httpx.WriteError(w, r, h.Logger, httpx.NewAPIError(http.StatusInternalServerError, httpx.CodeInternal, "unseal credentials", err))
		return
	}
	outgoing := mail.OutgoingMessage{
		From:       mail.Address{Email: creds.Email},
		To:         req.To,
		Cc:         req.Cc,
		Bcc:        req.Bcc,
		Subject:    req.Subject,
		BodyText:   req.BodyText,
		BodyHTML:   req.BodyHTML,
		InReplyTo:  req.InReplyTo,
		References: req.References,
	}
	if err := h.Sender.Send(r.Context(), creds, outgoing); err != nil {
		httpx.WriteError(w, r, h.Logger, httpx.NewAPIError(http.StatusBadGateway, httpx.CodeUpstreamFailed, "smtp send", err))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}
