// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package controller

import (
	"fmt"

	"github.com/hashicorp/consul/internal/controller/cache/index"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type watch struct {
	watchedType *pbresource.Type
	mapper      DependencyMapper
	indexes     map[string]*index.Index
}

func newWatch(watchedType *pbresource.Type, mapper DependencyMapper) *watch {
	if mapper == nil {
		panic("mapper not provided")
	}

	return &watch{
		watchedType: watchedType,
		indexes:     make(map[string]*index.Index),
		mapper:      mapper,
	}
}

func (w *watch) addIndex(index *index.Index) {
	if _, indexExists := w.indexes[index.Name()]; indexExists {
		panic(fmt.Sprintf("index with name %s is already defined", index.Name()))
	}

	w.indexes[index.Name()] = index
}

func (w *watch) key() string {
	return resource.ToGVK(w.watchedType)
}
