// proxy/vless/database/storage.go
package database

import (
	"context"

	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/uuid"
)

type UserStorage interface {
	GetUserByID(ctx context.Context, id uuid.UUID) (*protocol.MemoryUser, error)
	GetUserByEmail(ctx context.Context, email string) (*protocol.MemoryUser, error)
	GetUsers(ctx context.Context, offset, limit int) ([]*protocol.MemoryUser, error)
	AddUser(ctx context.Context, user *protocol.MemoryUser) error
	DelUser(ctx context.Context, email string) error
	GetCount(ctx context.Context) (int64, error)
	Close() error
}
