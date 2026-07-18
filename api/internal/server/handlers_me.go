package server

import (
	"net/http"

	"github.com/kubespaces-io/kubespaces/api/internal/auth"
)

// meResponse is the GET /me payload.
type meResponse struct {
	Subject string   `json:"subject"`
	Email   string   `json:"email"`
	Roles   []string `json:"roles"`
}

func handleMe(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "missing authentication")
		return
	}
	roles := claims.Roles
	if roles == nil {
		roles = []string{}
	}
	respondJSON(w, http.StatusOK, meResponse{
		Subject: claims.Subject,
		Email:   claims.Email,
		Roles:   roles,
	})
}
