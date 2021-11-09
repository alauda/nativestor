package v2

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSecurityContextConstraints(t *testing.T) {
	name := "topolvm"
	scc := NewSecurityContextConstraints(name, name)
	assert.True(t, scc.AllowPrivilegedContainer)
	assert.Equal(t, name, scc.Name)
}
