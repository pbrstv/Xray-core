package database

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/uuid"
	"github.com/xtls/xray-core/proxy/vless"
	"github.com/xtls/xray-core/proxy/vless/database/cache"
)

type Validator struct {
	storage UserStorage
	cache   *cache.Cache
}

func NewValidator(storage UserStorage, cacheSettings *CacheSettings) (*Validator, error) {
	validator := &Validator{
		storage: storage,
	}

	if cacheSettings != nil {
		ttl := time.Hour
		maxSize := int32(1000)

		ttl = time.Duration(cacheSettings.GetTtl()) * time.Second
		maxSize = cacheSettings.GetMaxSize()

		validator.cache = cache.NewCache(ttl, maxSize)
	}

	return validator, nil
}

func (v *Validator) Add(user *protocol.MemoryUser) error {
	_, ok := user.Account.(*vless.MemoryAccount)
	if !ok {
		return errors.New("not a VLESS user")
	}

	err := v.storage.AddUser(context.Background(), user)
	if err != nil {
		return err
	}

	if v.cache != nil {
		account, _ := user.Account.(*vless.MemoryAccount)
		uuidVal := account.ID.UUID()
		uuid := (&uuidVal).String()
		v.cache.Set(uuid, user)
	}

	return nil
}

func (v *Validator) Get(id uuid.UUID) *protocol.MemoryUser {
	uuid := (&id).String()

	if v.cache != nil {
		if user, exists := v.cache.Get(uuid); exists {
			return user
		}
	}

	user, err := v.storage.GetUserByID(context.Background(), id)
	if err != nil {
		return nil
	}

	if user != nil && v.cache != nil {
		v.cache.Set(uuid, user)
	}

	return user
}

func (v *Validator) GetByEmail(email string) *protocol.MemoryUser {
	if v.cache != nil {
		if user, exists := v.cache.GetByEmail(email); exists {
			return user
		}
	}

	user, err := v.storage.GetUserByEmail(context.Background(), email)
	if err != nil {
		return nil
	}

	if user != nil && v.cache != nil {
		account, ok := user.Account.(*vless.MemoryAccount)
		if ok {
			uuidVal := account.ID.UUID()
			uuid := (&uuidVal).String()
			v.cache.Set(uuid, user)
		}
	}

	return user
}

func (v *Validator) GetAll() []*protocol.MemoryUser {
	if v.cache != nil {
		return v.cache.GetAll()
	} else {
		var allUsers []*protocol.MemoryUser
		offset := 0
		limit := 100

		for {
			users, err := v.storage.GetUsers(context.Background(), offset, limit)
			if err != nil {
				return nil
			}

			if len(users) == 0 {
				break
			}

			allUsers = append(allUsers, users...)

			if len(users) < limit {
				break
			}

			offset += limit
		}

		return allUsers
	}
}

func (v *Validator) Del(email string) error {
	err := v.storage.DelUser(context.Background(), email)
	if err != nil {
		return err
	}

	if v.cache != nil {
		v.cache.DeleteByEmail(email)
	}

	return nil
}

func (v *Validator) GetCount() int64 {
	if v.cache != nil {
		return v.cache.GetCount()
	} else {
		count, err := v.storage.GetCount(context.Background())
		if err != nil {
			return 0
		}
		return count
	}
}

func (v *Validator) Close() error {
	if v.cache != nil {
		v.cache.Clear()
	}

	if closer, ok := v.storage.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}
