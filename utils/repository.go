package utils

import (
	"context"
	"fmt"
	"reflect"

	"github.com/uptrace/bun"
)

type Repository[TProto any, TModel any] interface {
	GetOne(ctx context.Context, id int64) (*TProto, error)
	Create(ctx context.Context, data *TProto) (*TProto, error)
	Update(ctx context.Context, data *TProto) (*TProto, error)
}

type repository[TProto any, TModel any] struct {
	db     *bun.DB
	mapper *GenericMapper
}

func NewRepository[TProto any, TModel any](db *bun.DB) Repository[TProto, TModel] {
	return &repository[TProto, TModel]{
		db:     db,
		mapper: NewMapper(nil),
	}
}

func (r repository[TProto, TModel]) GetOne(ctx context.Context, id int64) (*TProto, error) {
	data := new(TProto)
	var model TModel

	err := r.db.NewSelect().Model(&model).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, err
	}

	if err := r.mapper.MapStruct(model, data); err != nil {
		return nil, err
	}

	return data, nil
}

func (r repository[TProto, TModel]) Create(ctx context.Context, data *TProto) (*TProto, error) {
	model := new(TModel)

	if err := r.mapper.MapStruct(data, model); err != nil {
		return nil, err
	}

	if _, err := r.db.NewInsert().Model(model).Returning("*").Exec(ctx); err != nil {
		return nil, err
	}

	if err := r.mapper.MapStruct(model, data); err != nil {
		return nil, err
	}

	fmt.Printf("data: %v\n", data)

	return data, nil
}

func (r repository[TProto, TModel]) Update(ctx context.Context, data *TProto) (*TProto, error) {
	model := new(TModel)
	if err := r.mapper.MapStruct(data, model); err != nil {
		return nil, err
	}

	val := reflect.ValueOf(data).Elem()

	_, err := r.db.NewUpdate().
		Model(model).
		ExcludeColumn("created_at").
		Where("id = ?", val.FieldByName("Id")).
		Exec(ctx)
	if err != nil {
		return nil, err
	}

	return data, nil
}
