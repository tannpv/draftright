package feedback

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/tannpv/draftright-rewrite/internal/platform/auth"
	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// createService / listService / voteService are the handler's consumer-side
// ports (Service satisfies all three). One Service backs every route.
type feedbackService interface {
	CreateFeedback(ctx context.Context, in CreateInput, userID string) (Created, error)
	ListPublicFeatures(ctx context.Context, p ListParams, userID string) (ListResult, error)
	ToggleVote(ctx context.Context, featureID, userID string) (VoteResult, error)
}

// Handler serves the public feedback board:
//
//	POST /feedback           create a bug or feature request (JWT optional → user_id)
//	GET  /feedback           public board feed (kind=feature, is_public)
//	POST /feedback/:id/vote  toggle the caller's upvote (JWT enforced in-handler)
//
// All three read an OPTIONAL bearer via auth.OptionalUserID; vote rejects an
// absent identity itself (Node throws UnauthorizedException in the controller
// body, not via a guard), so none are mounted under hard RequireAuth.
type Handler struct {
	svc      feedbackService
	verifier *auth.Verifier // may be nil in tests
}

// NewHandler wires the service + the access-token verifier.
func NewHandler(svc *Service, v *auth.Verifier) *Handler { return &Handler{svc: svc, verifier: v} }

// createBody decodes the POST /feedback JSON. Pointers distinguish a missing key
// (nil) from an empty-string value, which the DTO @IsString / @MinLength checks
// depend on.
type createBody struct {
	Kind           *string `json:"kind"`
	Title          *string `json:"title"`
	TargetPlatform *string `json:"target_platform"`
	Description    *string `json:"description"`
	Source         *string `json:"source"`
	AppVersion     *string `json:"app_version"`
	OsInfo         *string `json:"os_info"`
	UserEmail      *string `json:"user_email"`
	Website        *string `json:"website"`
}

// honeypotResp is the silent-drop body. Field order matches Node exactly:
// { id, message }. id serialises as literal JSON null.
type honeypotResp struct {
	ID      *string `json:"id"`
	Message string  `json:"message"`
}

// createResp is the normal create body. Field order matches Node exactly:
// { id, ref, message }.
type createResp struct {
	ID      string `json:"id"`
	Ref     string `json:"ref"`
	Message string `json:"message"`
}

// Create handles POST /feedback → 201. Execution order mirrors NestJS:
// ValidationPipe (DTO) → controller honeypot → service.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var body createBody
	dec := json.NewDecoder(r.Body)
	// Node global pipe uses whitelist + forbidNonWhitelisted → an unknown body
	// property is a 400. Mirror the rewrite module's idiom (handler_rewrite.go):
	// DisallowUnknownFields, then collapse any decode failure to the generic
	// invalid-input 400 (status, not class-validator's exact "property X should
	// not exist" message). createBody has a json field for EVERY CreateFeedbackDto
	// property (kind, title, target_platform, description, source, app_version,
	// os_info, user_email, website) — including the `website` honeypot, which is a
	// whitelisted DTO field clients legitimately send, so it is NOT rejected here.
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		shared.WriteError(w, r, "invalid-input", "Invalid request body")
		return
	}

	flds := reqBody{
		Kind:           deref(body.Kind),
		Title:          deref(body.Title),
		TitlePresent:   body.Title != nil,
		TargetPlatform: deref(body.TargetPlatform),
		PlatformIsSet:  body.TargetPlatform != nil,
		Description:    deref(body.Description),
		DescPresent:    body.Description != nil,
		Source:         deref(body.Source),
		SourcePresent:  body.Source != nil,
		AppVersion:     deref(body.AppVersion),
		OsInfo:         deref(body.OsInfo),
		UserEmail:      deref(body.UserEmail),
		Website:        deref(body.Website),
	}

	// Stage: ValidationPipe (DTO constraints) — BEFORE the honeypot.
	if msg := validateFeedback(flds); msg != "" {
		shared.WriteError(w, r, "invalid-input", msg)
		return
	}

	// Controller honeypot: non-empty website → silent 201 drop, no row written,
	// distinct body { id:null, message:'Received. Thanks!' }.
	if strings.TrimSpace(flds.Website) != "" {
		shared.WriteJSON(w, http.StatusCreated, honeypotResp{ID: nil, Message: "Received. Thanks!"})
		return
	}

	userID := auth.OptionalUserID(h.verifier, r)
	created, err := h.svc.CreateFeedback(r.Context(), CreateInput{
		Kind:        flds.Kind,
		Title:       flds.Title,
		Platform:    flds.TargetPlatform,
		Description: flds.Description,
		Source:      flds.Source,
		AppVersion:  flds.AppVersion,
		OsInfo:      flds.OsInfo,
		UserEmail:   flds.UserEmail,
	}, userID)
	if err != nil {
		switch {
		case errors.Is(err, ErrDescriptionRequired),
			errors.Is(err, ErrSourceRequired),
			errors.Is(err, ErrTitleRequired),
			errors.Is(err, ErrBadTargetPlatform):
			shared.WriteError(w, r, "invalid-input", err.Error())
		case errors.Is(err, ErrFeatureNotFound):
			shared.WriteError(w, r, "not-found", err.Error())
		default:
			shared.WriteError(w, r, "internal", "feedback failed")
		}
		return
	}

	// formatDisplayNumber(kind, display_no): feature → FR-<n>, bug → BUG-<n>.
	prefix := "BUG-"
	noun := "Bug report"
	if created.Kind == "feature" {
		prefix = "FR-"
		noun = "Feature request"
	}
	ref := prefix + strconv.FormatInt(created.DisplayNo, 10)
	shared.WriteJSON(w, http.StatusCreated, createResp{
		ID:      created.ID,
		Ref:     ref,
		Message: noun + " received. Thanks! Reference: " + ref,
	})
}

// List handles GET /feedback → 200 { rows, total }. The status / target_platform
// allow-lists are applied here (nil = skip the filter) exactly as the Node
// controller/service does before querying.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	params := ListParams{
		Page:     atoiOrZero(q.Get("page")),
		Limit:    atoiOrZero(q.Get("limit")),
		Status:   statusFilter(q.Get("status")),
		Platform: platformFilter(q.Get("target_platform")),
	}

	userID := auth.OptionalUserID(h.verifier, r)
	res, err := h.svc.ListPublicFeatures(r.Context(), params, userID)
	if err != nil {
		shared.WriteError(w, r, "internal", "feedback failed")
		return
	}
	if res.Rows == nil {
		res.Rows = []FeatureRow{}
	}
	shared.WriteJSON(w, http.StatusOK, res)
}

// Vote handles POST /feedback/:id/vote → 200 { vote_count, hasVoted }. An absent
// identity → 401 (Node UnauthorizedException → AllExceptionsFilter code
// invalid-token); a missing/non-feature id → 404.
func (h *Handler) Vote(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := auth.OptionalUserID(h.verifier, r)
	if userID == "" {
		shared.WriteError(w, r, "invalid-token", "sign in to vote")
		return
	}
	res, err := h.svc.ToggleVote(r.Context(), id, userID)
	if err != nil {
		if errors.Is(err, ErrFeatureNotFound) {
			shared.WriteError(w, r, "not-found", err.Error())
			return
		}
		shared.WriteError(w, r, "internal", "feedback failed")
		return
	}
	shared.WriteJSON(w, http.StatusOK, res)
}

// ALLOWED_STATUSES mirrors the service constant. The board accepts a status
// filter only when it is non-empty, not 'all', and in this allow-list; anything
// else → nil (no filter), matching listPublicFeatures.
var allowedStatuses = map[string]bool{
	"new":          true,
	"reviewing":    true,
	"fix_proposed": true,
	"resolved":     true,
	"wont_fix":     true,
}

func statusFilter(s string) *string {
	if s == "" || s == "all" || !allowedStatuses[s] {
		return nil
	}
	return &s
}

// platformFilter applies TARGET_PLATFORMS the same way: keep only an in-list
// value, else nil (no filter).
func platformFilter(p string) *string {
	if p == "" || !PlatformValid(p) {
		return nil
	}
	return &p
}

// atoiOrZero mirrors JS Number(x) || default semantics for page/limit: a
// non-numeric or absent value → 0, which the service then defaults/clamps.
func atoiOrZero(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
