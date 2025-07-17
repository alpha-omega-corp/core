package models

type Permission struct {
	Id        int64 `json:"id" bun:",pk,autoincrement"`
	Read      bool  `json:"read" bun:"read"`
	Write     bool  `json:"write" bun:"write"`
	Manage    bool  `json:"manage" bun:"manage"`
	RoleId    int64
	ServiceID int64
}

type Service struct {
	Id          int64        `json:"id" bun:",pk,autoincrement"`
	Name        string       `json:"name" bun:"name,unique"`
	Permissions []Permission `bun:"rel:has-many,join:id=service_id"`
}
