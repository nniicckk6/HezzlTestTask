package model

import (
	"reflect"
	"testing"
)

func TestGoodDBTags(t *testing.T) {
	// получаем тип структуры Good для анализа рефлексией
	typ := reflect.TypeOf(Good{})
	// проверяем поле ID и его тег db
	field, found := typ.FieldByName("ID")
	if !found {
		t.Errorf("Поле ID не найдено в структуре Good")
	}
	// ожидаем, что в теге db указано "id"
	if field.Tag.Get("db") != "id" {
		t.Errorf("Ожидался тег db:'id' для поля ID, получили '%s'", field.Tag.Get("db"))
	}
	// проверяем поле ProjectID и его тег db
	field, _ = typ.FieldByName("ProjectID")
	// ожидаем, что тег db соответствует полю project_id в базе
	if field.Tag.Get("db") != "project_id" {
		t.Errorf("Ожидался тег db:'project_id' для поля ProjectID, получили '%s'", field.Tag.Get("db"))
	}
}

func TestPriorityUpdateDBTags(t *testing.T) {
	// получаем тип структуры PriorityUpdate
	typ := reflect.TypeOf(PriorityUpdate{})
	// проверяем поле ID на соответствие тега db
	field, found := typ.FieldByName("ID")
	if !found {
		t.Errorf("Поле ID не найдено в структуре PriorityUpdate")
	}
	if field.Tag.Get("db") != "id" {
		t.Errorf("Ожидался тег db:'id' для поля ID, получили '%s'", field.Tag.Get("db"))
	}
	// проверяем поле Priority и его тег db
	field, _ = typ.FieldByName("Priority")
	// ожидаем, что тег db соответствует столбцу priority в базе
	if field.Tag.Get("db") != "priority" {
		t.Errorf("Ожидался тег db:'priority' для поля Priority, получили '%s'", field.Tag.Get("db"))
	}
}
