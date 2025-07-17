package models

import "github.com/uptrace/bun"

type Role struct {
	bun.BaseModel `bun:"table:roles,alias:r"`
	Id            int64        `json:"id" bun:",pk,autoincrement"`
	Name          string       `json:"name" bun:"name,unique"`
	Permissions   []Permission `bun:"rel:has-many,join:id=role_id"`
}
