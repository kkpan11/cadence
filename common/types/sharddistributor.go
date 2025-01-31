// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package types

import "fmt"

type GetShardOwnerRequest struct {
	ShardKey  string
	Namespace string
}

func (v *GetShardOwnerRequest) GetShardKey() (o string) {
	if v != nil {
		return v.ShardKey
	}
	return
}

func (v *GetShardOwnerRequest) GetNamespace() (o string) {
	if v != nil {
		return v.Namespace
	}
	return
}

type GetShardOwnerResponse struct {
	Owner     string
	Namespace string
}

func (v *GetShardOwnerResponse) GetOwner() (o string) {
	if v != nil {
		return v.Owner
	}
	return
}

func (v *GetShardOwnerResponse) GetNamespace() (o string) {
	if v != nil {
		return v.Namespace
	}
	return
}

type NamespaceNotFoundError struct {
	Namespace string
}

func (n *NamespaceNotFoundError) Error() (o string) {
	if n != nil {
		return fmt.Sprintf("namespace not found %v", n.Namespace)
	}
	return
}
