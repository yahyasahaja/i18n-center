package mocks

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/lapakgaming/i18n-center/models"
)

func TestMockAuditServicer_Methods(t *testing.T) {
	m := NewMockAuditServicer(t)
	uid := uuid.New()
	rid := uuid.New()

	m.On("LogCreate", uid, "u", "component", rid, "code", "data", "ip", "ua").Return(nil)
	m.On("LogUpdate", uid, "u", "component", rid, "code", "before", "after", "ip", "ua").Return(nil)
	m.On("LogDelete", uid, "u", "component", rid, "code", "data", "ip", "ua").Return(nil)
	m.On("LogAction", uid, "u", "DEPLOY", "translation", rid, "code", map[string]interface{}{"a": 1}, "ip", "ua").Return(nil)
	m.On("GetAuditLogs", "component", rid, 10).Return([]models.AuditLog{}, nil)
	m.On("GetAuditLogsByUser", uid, 10).Return([]models.AuditLog{}, nil)
	m.On("GetChangesForResource", "component", rid).Return([]models.AuditLog{}, nil)

	assert.NoError(t, m.LogCreate(uid, "u", "component", rid, "code", "data", "ip", "ua"))
	assert.NoError(t, m.LogUpdate(uid, "u", "component", rid, "code", "before", "after", "ip", "ua"))
	assert.NoError(t, m.LogDelete(uid, "u", "component", rid, "code", "data", "ip", "ua"))
	assert.NoError(t, m.LogAction(uid, "u", "DEPLOY", "translation", rid, "code", map[string]interface{}{"a": 1}, "ip", "ua"))
	_, err := m.GetAuditLogs("component", rid, 10)
	assert.NoError(t, err)
	_, err = m.GetAuditLogsByUser(uid, 10)
	assert.NoError(t, err)
	_, err = m.GetChangesForResource("component", rid)
	assert.NoError(t, err)
}
