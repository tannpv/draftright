package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// SSE writer helpers. Kept in one file so the wire format is owned
// by a single place: any change here changes every SSE endpoint at
// once (Rule #1). Mirrors the OpenAI / Anthropic Server-Sent Events
// conventions so existing client SSE parsers keep working when we
// flip from one provider to another.
//
// Wire format:
//
//	data: {"delta":"<chunk>"}\n\n              ← every token
//	data: {"usage":{"input":N,"output":M,"ms":K}}\n\n  ← optional final
//	data: [DONE]\n\n                            ← terminal marker
//	event: error\ndata: {"error":"..."}\n\n    ← error stream
//
// The [DONE] marker is the same string OpenAI emits; reusing it means
// the mobile + desktop SSE parsers we already ship treat the Go
// service as a drop-in replacement.

// errStreamClosed is returned by the writer methods after the
// response has been flushed past the point where headers can change.
// Currently informational; future Task 9 metrics will count it.
var errStreamClosed = errors.New("sse: response stream closed")

// sseWriter wraps an http.ResponseWriter for SSE responses. Caller
// invokes Open() once to commit headers, then writeDelta / writeUsage /
// writeError / writeDone for each event. Every method flushes so the
// browser/client gets the chunk immediately — required for streaming
// UX.
type sseWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
	opened  bool
}

// newSSEWriter returns an SSE writer if the underlying ResponseWriter
// supports flushing. Returns nil + error when the server can't stream
// (rare — chi + stdlib server both flush; would only fail behind a
// non-streaming proxy).
func newSSEWriter(w http.ResponseWriter) (*sseWriter, error) {
	f, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("sse: response writer does not support flushing")
	}
	return &sseWriter{w: w, flusher: f}, nil
}

// Open commits SSE headers + sends a comment line to flush the
// response head. Browsers and `curl -N` start showing events only
// once the head has been seen — without this initial flush, the
// first delta can sit in a proxy buffer.
func (s *sseWriter) Open() {
	s.w.Header().Set("Content-Type", "text/event-stream")
	s.w.Header().Set("Cache-Control", "no-cache, no-transform")
	s.w.Header().Set("Connection", "keep-alive")
	// Nginx-friendly: disables proxy buffering on the response (Caddy
	// ignores it; harmless).
	s.w.Header().Set("X-Accel-Buffering", "no")
	s.w.WriteHeader(http.StatusOK)
	// Initial comment to flush the head past intermediaries.
	_, _ = s.w.Write([]byte(": ok\n\n"))
	s.flusher.Flush()
	s.opened = true
}

// writeDelta sends one token chunk. The body is JSON so the client
// can attach metadata later (cursor offset, attribution) without
// breaking the wire.
func (s *sseWriter) writeDelta(token string) error {
	return s.writeEvent("", map[string]string{"delta": token})
}

// writeUsage sends the final usage telemetry line so the client can
// surface cost / quota info to the user. Optional — handler may skip.
func (s *sseWriter) writeUsage(input, output, ms int32) error {
	return s.writeEvent("", map[string]any{
		"usage": map[string]int32{
			"input":  input,
			"output": output,
			"ms":     ms,
		},
	})
}

// writeError sends an SSE "error" event. Client SDKs that listen for
// onerror callbacks (EventSource API) get a clean signal separate
// from the data stream.
func (s *sseWriter) writeError(code, msg string) error {
	return s.writeEvent("error", map[string]string{
		"error": msg,
		"code":  code,
	})
}

// writeDone emits the terminal `data: [DONE]` marker matching the
// OpenAI convention. Flush after this and the response is complete.
func (s *sseWriter) writeDone() error {
	if _, err := fmt.Fprint(s.w, "data: [DONE]\n\n"); err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

// writeEvent is the low-level emitter. event="" omits the event-name
// line (default event type per the SSE spec); a non-empty string
// emits `event: <name>\n` before the data line.
func (s *sseWriter) writeEvent(event string, payload any) error {
	if !s.opened {
		return errStreamClosed
	}
	if event != "" {
		if _, err := fmt.Fprintf(s.w, "event: %s\n", event); err != nil {
			return err
		}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("sse: marshal payload: %w", err)
	}
	if _, err := fmt.Fprintf(s.w, "data: %s\n\n", body); err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}
