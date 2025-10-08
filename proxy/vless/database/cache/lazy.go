package cache

import (
	"sync"
	"time"

	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/proxy/vless"
)

type User struct {
	User      *protocol.MemoryUser
	ExpiresAt time.Time
	lruNode   *LRUNode
}

type Cache struct {
	usersByID    map[string]*User
	usersByEmail map[string]*User
	lru          *LRUManager
	mutex        sync.RWMutex
	ttl          time.Duration
	maxSize      int32
}

func NewCache(ttl time.Duration, maxSize int32) *Cache {
	return &Cache{
		usersByID:    make(map[string]*User),
		usersByEmail: make(map[string]*User),
		lru:          NewLRUManager(),
		ttl:          ttl,
		maxSize:      maxSize,
	}
}

func (c *Cache) Get(id string) (*protocol.MemoryUser, bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	user, exists := c.usersByID[id]
	if !exists {
		return nil, false
	}

	if time.Now().After(user.ExpiresAt) {
		if user.lruNode != nil {
			c.lru.Remove(user.lruNode)
		}
		delete(c.usersByID, id)
		delete(c.usersByEmail, user.User.Email)

		return nil, false
	}

	if user.lruNode != nil {
		c.lru.MoveToFront(user.lruNode)
	}

	return user.User, true
}

func (c *Cache) GetByEmail(email string) (*protocol.MemoryUser, bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	user, exists := c.usersByEmail[email]
	if !exists {
		return nil, false
	}

	account, ok := user.User.Account.(*vless.MemoryAccount)
	if !ok {
		return nil, false
	}

	uuidVal := account.ID.UUID()
	uuid := (&uuidVal).String()

	if time.Now().After(user.ExpiresAt) {
		if node, exists := c.lru.GetNode(uuid); exists {
			c.lru.Remove(node)
		}
		delete(c.usersByEmail, email)
		delete(c.usersByID, uuid)

		return nil, false
	}

	if node, exists := c.lru.GetNode(uuid); exists {
		c.lru.MoveToFront(node)
	}

	return user.User, true
}

func (c *Cache) Set(id string, user *protocol.MemoryUser) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.maxSize > 0 && int32(len(c.usersByID)) >= c.maxSize {
		// Remove the least recently used item (from the tail of the list)
		if c.lru.tail != nil {
			id := c.lru.tail.key
			if user, exists := c.usersByID[id]; exists {
				delete(c.usersByID, id)
				delete(c.usersByEmail, user.User.Email)
				c.lru.RemoveTail()
			}
		}
	}

	// Add a new user to the cache
	expiresAt := time.Now().Add(c.ttl)
	newUser := &User{User: user, ExpiresAt: expiresAt}
	c.usersByID[id] = newUser
	c.usersByEmail[user.Email] = newUser
	newUser.lruNode = c.lru.Add(id)

	return nil
}

func (c *Cache) Delete(id string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	user, exists := c.usersByID[id]
	if exists {
		delete(c.usersByID, id)
		delete(c.usersByEmail, user.User.Email)
		if user.lruNode != nil {
			c.lru.Remove(user.lruNode)
		}
	}
}

func (c *Cache) DeleteByEmail(email string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	user, exists := c.usersByEmail[email]
	if exists {
		delete(c.usersByEmail, email)

		// Also remove from cache by ID
		// If user contains an account with UUID, get ID directly
		account, ok := user.User.Account.(*vless.MemoryAccount)
		if ok {
			uuidVal := account.ID.UUID()
			uuid := (&uuidVal).String()
			delete(c.usersByID, uuid)
		} else {
			// If unable to get ID from account, search in usersByID by email
			// (this is a rare case, but just in case)
			for id, user := range c.usersByID {
				if user.User.Email == email {
					delete(c.usersByID, id)
					break // Assume email is unique
				}
			}
		}

		if user.lruNode != nil {
			c.lru.Remove(user.lruNode)
		}
	}
}

func (c *Cache) GetAll() []*protocol.MemoryUser {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	var users []*protocol.MemoryUser
	for _, user := range c.usersByID {
		if !time.Now().After(user.ExpiresAt) {
			users = append(users, user.User)
		}
	}

	return users
}

func (c *Cache) GetCount() int64 {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	count := 0
	for _, user := range c.usersByID {
		if !time.Now().After(user.ExpiresAt) {
			count++
		}
	}

	return int64(count)
}

func (c *Cache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.usersByID = make(map[string]*User)
	c.usersByEmail = make(map[string]*User)
	c.lru = NewLRUManager()
}
