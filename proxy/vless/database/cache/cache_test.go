package cache

import (
	"testing"
	"time"

	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/uuid"
	"github.com/xtls/xray-core/proxy/vless"
)

func TestCacheSetAndGet(t *testing.T) {
	cache := NewCache(5*time.Second, 100)

	// Create a test user
	id := "test-user-id"
	email := "test@example.com"
	account := &vless.MemoryAccount{
		ID: protocol.NewID(uuid.New()),
	}
	user := &protocol.MemoryUser{
		Email:   email,
		Account: account,
	}

	// Add user to cache
	err := cache.Set(id, user)
	if err != nil {
		t.Fatalf("Failed to set user: %v", err)
	}

	// Get user by ID
	user, exists := cache.Get(id)
	if !exists {
		t.Fatalf("User not found in cache")
	}

	if user.Email != email {
		t.Fatalf("Expected email %s, got %s", email, user.Email)
	}

	// Get user by email
	userByEmail, exists := cache.GetByEmail(email)
	if !exists {
		t.Fatalf("User not found by email in cache")
	}

	if userByEmail.Email != email {
		t.Fatalf("Expected email %s, got %s", email, userByEmail.Email)
	}
}

func TestCacheTTL(t *testing.T) {
	cache := NewCache(100*time.Millisecond, 100) // Short TTL for testing

	id := "ttl-test-id"
	email := "ttl@example.com"
	account := &vless.MemoryAccount{
		ID: protocol.NewID(uuid.New()),
	}
	user := &protocol.MemoryUser{
		Email:   email,
		Account: account,
	}

	// Add user to cache
	cache.Set(id, user)

	// Verify user exists
	_, exists := cache.Get(id)
	if !exists {
		t.Fatalf("User should exist initially")
	}

	// Wait for TTL to expire
	time.Sleep(200 * time.Millisecond)

	// User should not exist anymore
	_, exists = cache.Get(id)
	if exists {
		t.Fatalf("User should not exist after TTL expiration")
	}

	_, exists = cache.GetByEmail(email)
	if exists {
		t.Fatalf("User should not exist by email after TTL expiration")
	}
}

func TestCacheMaxSize(t *testing.T) {
	cache := NewCache(5*time.Second, 2) // Max size of 2

	// Add 3 users to exceed max size
	users := []struct {
		id    string
		email string
	}{
		{"id1", "email1@example.com"},
		{"id2", "email2@example.com"},
		{"id3", "email3@example.com"},
	}

	for _, u := range users {
		account := &vless.MemoryAccount{
			ID: protocol.NewID(uuid.New()),
		}
		user := &protocol.MemoryUser{
			Email:   u.email,
			Account: account,
		}
		cache.Set(u.id, user)
	}

	// The cache should have at most 2 users
	count := cache.GetCount()
	if count > 2 {
		t.Fatalf("Expected at most 2 users, got %d", count)
	}

	// Test that GetAll returns correct count
	allUsers := cache.GetAll()
	if int64(len(allUsers)) != count {
		t.Fatalf("GetAll count doesn't match GetCount: %d vs %d", len(allUsers), count)
	}
}

func TestCacheDelete(t *testing.T) {
	cache := NewCache(5*time.Second, 100)

	id := "delete-test-id"
	email := "delete@example.com"
	account := &vless.MemoryAccount{
		ID: protocol.NewID(uuid.New()),
	}
	user := &protocol.MemoryUser{
		Email:   email,
		Account: account,
	}

	cache.Set(id, user)

	// Verify user exists
	_, exists := cache.Get(id)
	if !exists {
		t.Fatalf("User should exist before deletion")
	}

	// Delete the user
	cache.Delete(id)

	// Verify user is deleted
	_, exists = cache.Get(id)
	if exists {
		t.Fatalf("User should not exist after deletion")
	}

	_, exists = cache.GetByEmail(email)
	if exists {
		t.Fatalf("User should not exist by email after deletion")
	}
}

func TestCacheClear(t *testing.T) {
	cache := NewCache(5*time.Second, 100)

	// Add some users
	for i := 0; i < 3; i++ {
		id := "clear-test-id-" + string(rune(i+'0'))
		email := "clear" + string(rune(i+'0')) + "@example.com"
		account := &vless.MemoryAccount{
			ID: protocol.NewID(uuid.New()),
		}
		user := &protocol.MemoryUser{
			Email:   email,
			Account: account,
		}
		cache.Set(id, user)
	}

	// Verify cache is not empty
	count := cache.GetCount()
	if count == 0 {
		t.Fatalf("Cache should not be empty initially")
	}

	// Clear the cache
	cache.Clear()

	// Verify cache is empty
	count = cache.GetCount()
	if count != 0 {
		t.Fatalf("Cache should be empty after clear, got %d users", count)
	}

	allUsers := cache.GetAll()
	if len(allUsers) != 0 {
		t.Fatalf("GetAll should return empty slice after clear, got %d users", len(allUsers))
	}
}

func TestLRUManager(t *testing.T) {
	lru := NewLRUManager()

	// Add nodes
	node1 := lru.Add("key1")
	node2 := lru.Add("key2")
	node3 := lru.Add("key3")

	// Verify all nodes are tracked
	if lru.head != node3 { // Most recently added should be at head
		t.Fatalf("Head should be the last added node")
	}

	if lru.tail != node1 { // First added should be at tail
		t.Fatalf("Tail should be the first added node")
	}

	// Move node1 to front
	lru.MoveToFront(node1)
	if lru.head != node1 {
		t.Fatalf("After moving to front, node1 should be at head")
	}

	// Remove node
	lru.Remove(node2)
	_, exists := lru.GetNode("key2")
	if exists {
		t.Fatalf("Node2 should not exist after removal")
	}

	// Remove from tail
	lru.RemoveTail()
	_, exists = lru.GetNode("key3") // This should be the tail now
	if exists {
		t.Fatalf("Tail node should not exist after RemoveTail")
	}
}

func TestCacheDeleteByEmail(t *testing.T) {
	cache := NewCache(5*time.Second, 100)

	// Create user with ID from account, similar to how validator works
	account := &vless.MemoryAccount{
		ID: protocol.NewID(uuid.New()),
	}
	user := &protocol.MemoryUser{
		Email:   "delete-by-email@example.com",
		Account: account,
	}
	
	// Use the UUID string as the ID, similar to validator.Add method
	uuidVal := account.ID.UUID()
	id := (&uuidVal).String()

	cache.Set(id, user)

	// Verify user exists by both ID and email
	_, existsByID := cache.Get(id)
	if !existsByID {
		t.Fatalf("User should exist by ID before deletion")
	}

	_, existsByEmail := cache.GetByEmail(user.Email)
	if !existsByEmail {
		t.Fatalf("User should exist by email before deletion")
	}

	// Delete the user by email
	cache.DeleteByEmail(user.Email)

	// Verify user is deleted by both ID and email
	_, existsAfterID := cache.Get(id)
	if existsAfterID {
		t.Fatalf("User should not exist by ID after deletion by email")
	}

	_, existsAfterEmail := cache.GetByEmail(user.Email)
	if existsAfterEmail {
		t.Fatalf("User should not exist by email after deletion by email")
	}
}

func TestCacheDeleteByEmailNonExistent(t *testing.T) {
	cache := NewCache(5*time.Second, 100)

	// Try to delete a non-existent user by email - should not cause panic
	cache.DeleteByEmail("non-existent-email@example.com")
	
	// Cache should remain empty
	count := cache.GetCount()
	if count != 0 {
		t.Fatalf("Cache should be empty after deleting non-existent user, got %d users", count)
	}
}
