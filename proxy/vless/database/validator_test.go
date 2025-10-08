package database

import (
	"context"
	"testing"

	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/uuid"
	"github.com/xtls/xray-core/proxy/vless"
)

// MockUserStorage implements the UserStorage interface for testing
type MockUserStorage struct {
	users map[string]*protocol.MemoryUser
}

func NewMockUserStorage() *MockUserStorage {
	return &MockUserStorage{
		users: make(map[string]*protocol.MemoryUser),
	}
}

func (m *MockUserStorage) AddUser(ctx context.Context, user *protocol.MemoryUser) error {
	account, ok := user.Account.(*vless.MemoryAccount)
	if ok {
		uuidVal := account.ID.UUID()
		id := (&uuidVal).String()
		m.users[id] = user
	}
	return nil
}

func (m *MockUserStorage) GetUserByID(ctx context.Context, id uuid.UUID) (*protocol.MemoryUser, error) {
	idStr := (&id).String()
	user, exists := m.users[idStr]
	if !exists {
		return nil, nil
	}
	return user, nil
}

func (m *MockUserStorage) GetUserByEmail(ctx context.Context, email string) (*protocol.MemoryUser, error) {
	for _, user := range m.users {
		if user.Email == email {
			return user, nil
		}
	}
	return nil, nil
}

func (m *MockUserStorage) GetUsers(ctx context.Context, offset int, limit int) ([]*protocol.MemoryUser, error) {
	var users []*protocol.MemoryUser
	i := 0
	for _, user := range m.users {
		if i >= offset && len(users) < limit {
			users = append(users, user)
		}
		if len(users) >= limit {
			break
		}
		i++
	}
	return users, nil
}

func (m *MockUserStorage) DelUser(ctx context.Context, email string) error {
	// Find and remove user by email
	for id, user := range m.users {
		if user.Email == email {
			delete(m.users, id)
			break
		}
	}
	return nil
}

func (m *MockUserStorage) GetCount(ctx context.Context) (int64, error) {
	return int64(len(m.users)), nil
}

func (m *MockUserStorage) Close() error {
	return nil
}

func TestValidatorWithCache(t *testing.T) {
	storage := NewMockUserStorage()
	cacheSettings := &CacheSettings{
		Ttl:     300, // 5 minutes
		MaxSize: 100,
	}
	
	validator, err := NewValidator(storage, cacheSettings)
	if err != nil {
		t.Fatalf("Failed to create validator with cache: %v", err)
	}
	
	// Create a test user
	account := &vless.MemoryAccount{
		ID: protocol.NewID(uuid.New()),
	}
	user := &protocol.MemoryUser{
		Email:   "test@example.com",
		Account: account,
	}
	
	// Add the user
	err = validator.Add(user)
	if err != nil {
		t.Fatalf("Failed to add user: %v", err)
	}
	
	// Check that the user is available by ID
	uuidVal := account.ID.UUID()
	retrievedUser := validator.Get(uuidVal)
	if retrievedUser == nil {
		t.Fatalf("User not found by ID")
	}
	if retrievedUser.Email != user.Email {
		t.Fatalf("Wrong email returned, expected %s, got %s", user.Email, retrievedUser.Email)
	}
	
	// Check that the user is available by email
	retrievedUserByEmail := validator.GetByEmail(user.Email)
	if retrievedUserByEmail == nil {
		t.Fatalf("User not found by email")
	}
	if retrievedUserByEmail.Email != user.Email {
		t.Fatalf("Wrong email returned by GetByEmail, expected %s, got %s", user.Email, retrievedUserByEmail.Email)
	}
	
	// Remove the user
	err = validator.Del(user.Email)
	if err != nil {
		t.Fatalf("Failed to delete user: %v", err)
	}
	
	// Check that the user has been deleted
	uuidVal2 := account.ID.UUID()
	retrievedUserAfterDelete := validator.Get(uuidVal2)
	if retrievedUserAfterDelete != nil {
		t.Fatalf("User should be deleted by ID")
	}
	
	retrievedUserByEmailAfterDelete := validator.GetByEmail(user.Email)
	if retrievedUserByEmailAfterDelete != nil {
		t.Fatalf("User should be deleted by email")
	}
}

func TestValidatorWithoutCache(t *testing.T) {
	storage := NewMockUserStorage()
	
	validator, err := NewValidator(storage, nil)
	if err != nil {
		t.Fatalf("Failed to create validator without cache: %v", err)
	}
	
	// Create a test user
	account := &vless.MemoryAccount{
		ID: protocol.NewID(uuid.New()),
	}
	user := &protocol.MemoryUser{
		Email:   "test-no-cache@example.com",
		Account: account,
	}
	
	// Add the user
	err = validator.Add(user)
	if err != nil {
		t.Fatalf("Failed to add user: %v", err)
	}
	
	// Check that the user is available by ID (should be retrieved from the database)
	uuidVal3 := account.ID.UUID()
	retrievedUser := validator.Get(uuidVal3)
	if retrievedUser == nil {
		t.Fatalf("User not found by ID")
	}
	if retrievedUser.Email != user.Email {
		t.Fatalf("Wrong email returned, expected %s, got %s", user.Email, retrievedUser.Email)
	}
	
	// Check that the user is available by email (should be retrieved from the database)
	retrievedUserByEmail := validator.GetByEmail(user.Email)
	if retrievedUserByEmail == nil {
		t.Fatalf("User not found by email")
	}
	if retrievedUserByEmail.Email != user.Email {
		t.Fatalf("Wrong email returned by GetByEmail, expected %s, got %s", user.Email, retrievedUserByEmail.Email)
	}
	
	// Remove the user
	err = validator.Del(user.Email)
	if err != nil {
		t.Fatalf("Failed to delete user: %v", err)
	}
	
	// Check that the user has been deleted from the database
	uuidVal4 := account.ID.UUID()
	retrievedUserAfterDelete := validator.Get(uuidVal4)
	if retrievedUserAfterDelete != nil {
		t.Fatalf("User should be deleted by ID")
	}
	
	retrievedUserByEmailAfterDelete := validator.GetByEmail(user.Email)
	if retrievedUserByEmailAfterDelete != nil {
		t.Fatalf("User should be deleted by email")
	}
}

func TestValidatorGetAllAndCount(t *testing.T) {
	storage := NewMockUserStorage()
	
	validator, err := NewValidator(storage, nil)
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}
	
	// Add multiple users
	users := make([]*protocol.MemoryUser, 3)
	for i := 0; i < 3; i++ {
		account := &vless.MemoryAccount{
			ID: protocol.NewID(uuid.New()),
		}
		users[i] = &protocol.MemoryUser{
			Email:   "test" + string(rune(i+'0')) + "@example.com",
			Account: account,
		}
		
		err = validator.Add(users[i])
		if err != nil {
			t.Fatalf("Failed to add user %d: %v", i, err)
		}
	}
	
	// Check the number of users
	count := validator.GetCount()
	if count != 3 {
		t.Fatalf("Expected count 3, got %d", count)
	}
	
	// Check retrieval of all users
	allUsers := validator.GetAll()
	if int64(len(allUsers)) != count {
		t.Fatalf("Expected all users count %d, got %d", count, len(allUsers))
	}
	
	// Remove one user
	err = validator.Del(users[0].Email)
	if err != nil {
		t.Fatalf("Failed to delete user: %v", err)
	}
	
	// Check the updated count
	newCount := validator.GetCount()
	if newCount != 2 {
		t.Fatalf("Expected count 2 after deletion, got %d", newCount)
	}
	
	// Check the updated list
	newAllUsers := validator.GetAll()
	if int64(len(newAllUsers)) != newCount {
		t.Fatalf("Expected all users count %d after deletion, got %d", newCount, len(newAllUsers))
	}
}