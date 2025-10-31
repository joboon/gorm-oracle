/*
** Copyright (c) 2025 Oracle and/or its affiliates.
**
** The Universal Permissive License (UPL), Version 1.0
**
** Subject to the condition set forth below, permission is hereby granted to any
** person obtaining a copy of this software, associated documentation and/or data
** (collectively the "Software"), free of charge and under any and all copyright
** rights in the Software, and any and all patent rights owned or freely
** licensable by each licensor hereunder covering either (i) the unmodified
** Software as contributed to or provided by such licensor, or (ii) the Larger
** Works (as defined below), to deal in both
**
** (a) the Software, and
** (b) any piece of software and/or hardware listed in the lrgrwrks.txt file if
** one is included with the Software (each a "Larger Work" to which the Software
** is contributed by such licensors),
**
** without restriction, including without limitation the rights to copy, create
** derivative works of, display, perform, and distribute the Software and make,
** use, sell, offer for sale, import, export, have made, and have sold the
** Software and the Larger Work(s), and to sublicense the foregoing rights on
** either these or other terms.
**
** This license is subject to the following condition:
** The above copyright notice and either this complete permission notice or at
** a minimum a reference to the UPL must be included in all copies or
** substantial portions of the Software.
**
** THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
** IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
** FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
** AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
** LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
** OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
** SOFTWARE.
 */

package tests

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"gorm.io/gorm/clause"
)

type ClobOneToManyModel struct {
	ID       uint             `gorm:"primaryKey"`
	Children []ClobChildModel `gorm:"foreignKey:ParentID"`
}

type ClobChildModel struct {
	ParentID uint
	Blah     string `gorm:"primaryKey"`
	Data     string `gorm:"type:clob"`
}

type ClobSingleModel struct {
	ID   uint   `gorm:"primaryKey"`
	Data string `gorm:"type:clob"`
}

type BlobJSONModel struct {
	Blah string       `gorm:"primaryKey"`
	Data AttributeMap `gorm:"type:json"`
}

type BlobOneToManyModel struct {
	ID       uint             `gorm:"primaryKey"`
	Children []BlobChildModel `gorm:"foreignKey:ParentID"`
}

type BlobChildModel struct {
	ParentID uint
	Blah     string `gorm:"primaryKey"`
	Data     []byte `gorm:"type:blob"`
}

type BlobSingleModel struct {
	ID   uint   `gorm:"primaryKey"`
	Data []byte `gorm:"type:blob"`
}

func scanBytes(src interface{}) ([]byte, bool) {
	if stringer, ok := src.(fmt.Stringer); ok {
		return []byte(stringer.String()), true
	}
	bytes, ok := src.([]byte)
	if !ok {
		return nil, false
	}
	return bytes, true
}

type AttributeMap map[string]interface{}

func (a AttributeMap) Value() (driver.Value, error) {
	attrs := a
	if attrs == nil {
		attrs = AttributeMap{}
	}
	value, err := json.Marshal(attrs)
	return value, err
}

func (a *AttributeMap) Scan(src interface{}) error {
	bytes, ok := scanBytes(src)
	if !ok {
		return fmt.Errorf("failed to scan attribute map")
	}
	var raw interface{}
	err := json.Unmarshal(bytes, &raw)
	if err != nil {
		return err
	}

	if raw == nil {
		*a = map[string]interface{}{}
		return nil
	}
	*a, ok = raw.(map[string]interface{})
	if !ok {
		return fmt.Errorf("failed to convert attribute map from json")
	}
	return nil
}

func setupLobTestTables(t *testing.T) {
	t.Log("Setting up LOB test tables")

	DB.Migrator().DropTable(&ClobOneToManyModel{}, &ClobChildModel{}, &ClobSingleModel{}, &BlobOneToManyModel{}, &BlobChildModel{}, &BlobSingleModel{}, &BlobJSONModel{})

	err := DB.AutoMigrate(&ClobOneToManyModel{}, &ClobChildModel{}, &ClobSingleModel{}, &BlobOneToManyModel{}, &BlobChildModel{}, &BlobSingleModel{}, &BlobJSONModel{})
	if err != nil {
		t.Fatalf("Failed to migrate LOB test tables: %v", err)
	}

	t.Log("LOB test tables created successfully")
}

func TestClobOnConflict(t *testing.T) {
	type test struct {
		model any
		fn    func(model any) error
	}
	tests := map[string]test{
		"OneToManySingle": {
			model: &ClobOneToManyModel{
				ID: 1,
				Children: []ClobChildModel{
					{
						Blah: "1",
						Data: strings.Repeat("X", 32768),
					},
				},
			},
			fn: func(model any) error {
				return DB.Create(model).Error
			},
		},
		"OneToManyBatch": {
			model: &ClobOneToManyModel{
				ID: 1,
				Children: []ClobChildModel{
					{
						Blah: "1",
						Data: strings.Repeat("X", 32768),
					},
					{
						Blah: "2",
						Data: strings.Repeat("Y", 3),
					},
				},
			},
			fn: func(model any) error {
				return DB.Create(model).Error
			},
		},
		"Single": {
			model: []ClobSingleModel{
				{
					ID:   1,
					Data: strings.Repeat("X", 32768),
				},
			},
			fn: func(model any) error {
				return DB.Clauses(clause.OnConflict{
					UpdateAll: true,
				}).CreateInBatches(model, 1000).Error
			},
		},
		"SingleBatch": {
			model: []ClobSingleModel{
				{
					ID:   1,
					Data: strings.Repeat("X", 32768),
				},
				{
					ID:   2,
					Data: strings.Repeat("Y", 3),
				},
			},
			fn: func(model any) error {
				return DB.Clauses(clause.OnConflict{
					UpdateAll: true,
				}).CreateInBatches(model, 1000).Error
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			setupLobTestTables(t)
			err := tc.fn(tc.model)
			if err != nil {
				t.Fatalf("Failed to create CLOB record with ON CONFLICT: %v", err)
			}
		})
	}
}

func TestJSONBAsCLOB(t *testing.T) {
	type test struct {
		model any
		fn    func(model any) error
	}
	tests := map[string]test{
		"Single": {
			model: []BlobJSONModel{
				{
					Blah: "1",
					Data: AttributeMap{"Data": strings.Repeat("X", 3)},
				},
			},
			fn: func(model any) error {
				return DB.Clauses(clause.OnConflict{
					UpdateAll: true,
				}).CreateInBatches(model, 1000).Error
			},
		},
		"SingleBatch": {
			model: []BlobJSONModel{
				{
					Blah: "1",
					Data: AttributeMap{"Data": strings.Repeat("X", 32768)},
				},
				{
					Blah: "2",
					Data: AttributeMap{"Data": strings.Repeat("Y", 3)},
				},
			},
			fn: func(model any) error {
				return DB.Clauses(clause.OnConflict{
					UpdateAll: true,
				}).CreateInBatches(model, 1000).Error
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			setupLobTestTables(t)
			err := tc.fn(tc.model)
			if err != nil {
				t.Fatalf("Failed to create CLOB record with ON CONFLICT: %v", err)
			}
		})
	}
}

func TestBlobOnConflict(t *testing.T) {
	type test struct {
		model any
		fn    func(model any) error
	}
	tests := map[string]test{
		"OneToManySingle": {
			model: &BlobOneToManyModel{
				ID: 1,
				Children: []BlobChildModel{
					{
						Blah: "1",
						Data: []byte(strings.Repeat("X", 32768)),
					},
				},
			},
			fn: func(model any) error {
				return DB.Create(model).Error
			},
		},
		"OneToManyBatch": {
			model: &BlobOneToManyModel{
				ID: 1,
				Children: []BlobChildModel{
					{
						Blah: "1",
						Data: []byte(strings.Repeat("X", 32768)),
					},
					{
						Blah: "2",
						Data: []byte(strings.Repeat("Y", 3)),
					},
				},
			},
			fn: func(model any) error {
				return DB.Create(model).Error
			},
		},
		"Single": {
			model: []BlobSingleModel{
				{
					ID:   1,
					Data: []byte(strings.Repeat("X", 32768)),
				},
			},
			fn: func(model any) error {
				return DB.Clauses(clause.OnConflict{
					UpdateAll: true,
				}).CreateInBatches(model, 1000).Error
			},
		},
		"SingleBatch": {
			model: []BlobSingleModel{
				{
					ID:   1,
					Data: []byte(strings.Repeat("X", 32768)),
				},
				{
					ID:   2,
					Data: []byte(strings.Repeat("Y", 3)),
				},
			},
			fn: func(model any) error {
				return DB.Clauses(clause.OnConflict{
					UpdateAll: true,
				}).CreateInBatches(model, 1000).Error
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			setupLobTestTables(t)
			err := tc.fn(tc.model)
			if err != nil {
				t.Fatalf("Failed to create BLOB record with ON CONFLICT: %v", err)
			}
		})
	}
}
