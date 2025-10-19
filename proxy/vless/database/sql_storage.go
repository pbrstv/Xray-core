package database

import (
	"context"
	"database/sql"

	"github.com/uptrace/bun"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/uuid"
	"github.com/xtls/xray-core/proxy/vless"
)

// SQLStorage implements UserStorage for SQL databases.
type SQLStorage struct {
	db        *bun.DB
	tableName string
	adapter   SQLDriver
}

type sqlUserModel struct {
	bun.BaseModel `bun:"table:vless_users,alias:vu"`

	ID    string `bun:"id,pk"`
	Email string `bun:"email"`
	Flow  string `bun:"flow"`
}

func NewSQLStorage(cs *ClientsStorage) (UserStorage, error) {
	settings := cs.GetSettings()
	dsn := settings.GetDsn()
	tableName := settings.GetTable()
	poolSize := int(settings.GetPool())

	var adapter SQLDriver

	switch cs.Type {
	case "postgres":
		adapter = &PostgresDriver{}
	case "mysql":
		adapter = &MySQLDriver{}
	default:
		return nil, errors.New("unsupported SQL driver: " + cs.Type).AtError()
	}

	db, err := adapter.Connect(dsn, poolSize)
	if err != nil {
		return nil, errors.New("failed to connect to " + adapter.GetType() + " database").Base(err).AtError()
	}

	s := &SQLStorage{db: db, tableName: tableName, adapter: adapter}

	errors.LogInfo(context.Background(), "Successfully connected to database: ", cs.Type, " with table: ", tableName)
	return s, nil
}

func (s *SQLStorage) GetUserByID(ctx context.Context, id uuid.UUID) (*protocol.MemoryUser, error) {
	errors.LogDebug(ctx, "Searching for user with UUID string: ", id.String(), " in table: ", s.tableName)
	var model sqlUserModel
	err := s.db.NewSelect().Model(&model).ModelTableExpr(s.tableName+" AS vu").Where("id = ?", id.String()).Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			errors.LogDebug(ctx, "User with UUID ", id.String(), " not found in database (no rows returned)")
			return nil, errors.New("user not found with ID: ", id.String()).AtDebug()
		}

		if s.adapter.IsConnectionError(err) {
			return nil, errors.New("database connection error when getting user by ID: ", id.String()).Base(err).AtError()
		}

		errors.LogDebug(ctx, "Database query error when searching for user ", id.String(), ": ", err)
		return nil, errors.New("database query error when getting user by ID: ", id.String()).Base(err).AtError()
	}

	errors.LogDebug(ctx, "User with UUID ", id.String(), " found in database")
	return toMemoryUser(&model)
}

func (s *SQLStorage) GetUserByEmail(ctx context.Context, email string) (*protocol.MemoryUser, error) {
	var model sqlUserModel
	err := s.db.NewSelect().Model(&model).ModelTableExpr(s.tableName+" AS vu").Where("email = ?", email).Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("user not found with email: ", email).AtDebug()
		}

		if s.adapter.IsConnectionError(err) {
			return nil, errors.New("database connection error when getting user by email: ", email).Base(err).AtError()
		}

		return nil, errors.New("database query error when getting user by email: ", email).Base(err).AtError()
	}

	return toMemoryUser(&model)
}

func (s *SQLStorage) GetUsers(ctx context.Context, offset, limit int) ([]*protocol.MemoryUser, error) {
	var models []*sqlUserModel

	err := s.db.NewSelect().Model(&models).ModelTableExpr(s.tableName + " AS vu").Offset(offset).Limit(limit).Scan(ctx)
	if err != nil {
		if s.adapter.IsConnectionError(err) {
			return nil, errors.New("database connection error when getting users").Base(err).AtError()
		}
		return nil, errors.New("database query error when getting users").Base(err).AtError()
	}

	users := make([]*protocol.MemoryUser, len(models))
	for i, model := range models {
		user, err := toMemoryUser(model)
		if err != nil {
			return nil, errors.New("invalid user model data in position ", i).Base(err).AtError()
		}
		users[i] = user
	}

	return users, nil
}

func (s *SQLStorage) AddUser(ctx context.Context, user *protocol.MemoryUser) error {
	account, ok := user.Account.(*vless.MemoryAccount)
	if !ok {
		return errors.New("not a VLESS user").AtError()
	}

	id := account.ID.UUID()

	model := &sqlUserModel{ID: id.String(), Email: user.Email, Flow: account.Flow}

	_, err := s.db.NewInsert().Model(model).ModelTableExpr(s.tableName + " AS vu").Exec(ctx)
	if err != nil {
		if s.adapter.IsDuplicateKeyError(err) {
			return errors.New("user already exists").Base(err).AtWarning()
		}
		return errors.New("database insert error").Base(err).AtError()
	} else {
		errors.LogDebug(ctx, "Successfully added user to database")
	}

	return nil
}

func (s *SQLStorage) DelUser(ctx context.Context, email string) error {
	result, err := s.db.NewDelete().Model((*sqlUserModel)(nil)).ModelTableExpr(s.tableName+" AS vu").Where("email = ?", email).Exec(ctx)
	if err != nil {
		if s.adapter.IsConnectionError(err) {
			return errors.New("database connection error when deleting user with email: ", email).Base(err).AtError()
		}

		return errors.New("database delete error when deleting user with email: ", email).Base(err).AtError()
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		errors.LogDebug(ctx, "No user found to delete with email: ", email)
	}

	return nil
}

func (s *SQLStorage) GetCount(ctx context.Context) (int64, error) {
	count, err := s.db.NewSelect().Model((*sqlUserModel)(nil)).ModelTableExpr(s.tableName + " AS vu").Count(ctx)
	if err != nil {
		if s.adapter.IsConnectionError(err) {
			return 0, errors.New("database connection error when getting count").Base(err).AtError()
		}

		return 0, errors.New("database count query error").Base(err).AtError()
	}

	return int64(count), nil
}

func (s *SQLStorage) Close() error {
	err := s.db.Close()
	if err != nil {
		return errors.New("failed to close database connection").Base(err).AtError()
	}

	return nil
}

func toMemoryUser(model *sqlUserModel) (*protocol.MemoryUser, error) {
	uuid, err := uuid.ParseString(model.ID)
	if err != nil {
		return nil, errors.New("invalid UUID in database: " + model.ID).Base(err).AtError()
	}

	account := &vless.MemoryAccount{
		ID:         protocol.NewID(uuid),
		Flow:       model.Flow,
		Encryption: "none",
	}
	user := &protocol.MemoryUser{
		Email:   model.Email,
		Level:   0,
		Account: account,
	}

	return user, nil
}
