package models

import (
	"github.com/uptrace/bun"
	"time"
)

type User struct {
	bun.BaseModel `bun:"table:users,alias:u"`

	Id        int64     `json:"id" bun:",pk,autoincrement"`
	Name      string    `json:"name" bun:"name"`
	Email     string    `json:"email" bun:"email,unique"`
	Password  string    `json:"-" bun:"encrypted_password"`
	Roles     []Role    `bun:"m2m:user_to_roles,join:User=Role"`
	CreatedAt time.Time `bun:",nullzero,notnull,default:current_timestamp"`
	UpdatedAt time.Time `bun:",nullzero,notnull,default:current_timestamp"`
}

type UserToRole struct {
	UserID int64 `bun:",pk"`
	User   *User `bun:"rel:belongs-to,join:user_id=id"`
	RoleID int64 `bun:",pk"`
	Role   *Role `bun:"rel:belongs-to,join:role_id=id"`
}
