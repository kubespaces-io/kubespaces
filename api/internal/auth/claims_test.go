package auth

import (
	"testing"
)

func TestParseClaims(t *testing.T) {
	tests := []struct {
		name        string
		payload     string
		wantErr     bool
		wantSubject string
		wantEmail   string
		wantRoles   []string
	}{
		{
			name: "full keycloak claims",
			payload: `{
				"sub": "1234-abcd",
				"email": "alice@example.com",
				"realm_access": {"roles": ["kubespaces-member", "offline_access"]}
			}`,
			wantSubject: "1234-abcd",
			wantEmail:   "alice@example.com",
			wantRoles:   []string{"kubespaces-member", "offline_access"},
		},
		{
			name:        "no realm_access",
			payload:     `{"sub": "1234", "email": "bob@example.com"}`,
			wantSubject: "1234",
			wantEmail:   "bob@example.com",
			wantRoles:   nil,
		},
		{
			name:        "no email",
			payload:     `{"sub": "1234", "realm_access": {"roles": ["kubespaces-admin"]}}`,
			wantSubject: "1234",
			wantRoles:   []string{"kubespaces-admin"},
		},
		{name: "missing subject", payload: `{"email": "x@example.com"}`, wantErr: true},
		{name: "invalid json", payload: `{`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			claims, err := ParseClaims([]byte(tt.payload))

			// Assert
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseClaims() expected error, got claims %+v", claims)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseClaims() unexpected error: %v", err)
			}
			if claims.Subject != tt.wantSubject {
				t.Errorf("Subject = %q, want %q", claims.Subject, tt.wantSubject)
			}
			if claims.Email != tt.wantEmail {
				t.Errorf("Email = %q, want %q", claims.Email, tt.wantEmail)
			}
			if len(claims.Roles) != len(tt.wantRoles) {
				t.Fatalf("Roles = %v, want %v", claims.Roles, tt.wantRoles)
			}
			for i, role := range tt.wantRoles {
				if claims.Roles[i] != role {
					t.Errorf("Roles[%d] = %q, want %q", i, claims.Roles[i], role)
				}
			}
		})
	}
}

func TestClaimsIsAdmin(t *testing.T) {
	tests := []struct {
		name  string
		roles []string
		want  bool
	}{
		{name: "admin role present", roles: []string{"kubespaces-admin"}, want: true},
		{name: "admin among others", roles: []string{"offline_access", "kubespaces-admin"}, want: true},
		{name: "member only", roles: []string{"kubespaces-member"}, want: false},
		{name: "no roles", roles: nil, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			claims := &Claims{Subject: "s", Roles: tt.roles}

			// Act + Assert
			if got := claims.IsAdmin(); got != tt.want {
				t.Errorf("IsAdmin() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClaimsIdentity(t *testing.T) {
	tests := []struct {
		name   string
		claims Claims
		want   string
	}{
		{name: "prefers email", claims: Claims{Subject: "sub-1", Email: "a@example.com"}, want: "a@example.com"},
		{name: "falls back to subject", claims: Claims{Subject: "sub-1"}, want: "sub-1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.claims.Identity(); got != tt.want {
				t.Errorf("Identity() = %q, want %q", got, tt.want)
			}
		})
	}
}
