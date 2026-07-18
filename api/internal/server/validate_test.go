package server

import (
	"strings"
	"testing"
)

func TestValidateTenantName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "simple lowercase", input: "team-a", wantErr: false},
		{name: "single char", input: "a", wantErr: false},
		{name: "digits only", input: "123", wantErr: false},
		{name: "max length 40", input: strings.Repeat("a", 40), wantErr: false},
		{name: "empty", input: "", wantErr: true},
		{name: "too long 41", input: strings.Repeat("a", 41), wantErr: true},
		{name: "uppercase", input: "Team", wantErr: true},
		{name: "leading dash", input: "-team", wantErr: true},
		{name: "trailing dash", input: "team-", wantErr: true},
		{name: "underscore", input: "team_a", wantErr: true},
		{name: "dot", input: "team.a", wantErr: true},
		{name: "space", input: "team a", wantErr: true},
		{name: "unicode", input: "tëam", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			err := ValidateTenantName(tt.input)

			// Assert
			if gotErr := err != nil; gotErr != tt.wantErr {
				t.Errorf("ValidateTenantName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}
