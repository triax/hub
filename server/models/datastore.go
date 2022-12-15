package models

import (
	"cloud.google.com/go/datastore"
)

const (
	KindMember  = "Member"
	KindEvent   = "Event"
	KindEquip   = "Equip"
	KindCustody = "Custody"
)

// IsFieldMismatch ...
// datastoreのGet系メソッド利用時において、
// datastore側で存在するフィールドを、struct側が持っていない場合、
// ErrFieldMismatchが起きるが、これはdataのマイグレーション上めんどくさいので、
// このエラーだけは無視したいことが多々ある。
// @See
// 	- https://github.com/googleapis/google-cloud-go/issues/913
//	- https://pkg.go.dev/cloud.google.com/go/datastore#ErrFieldMismatch
func IsFiledMismatch(err error) bool {
	_, ok := err.(*datastore.ErrFieldMismatch)
	return ok
}
