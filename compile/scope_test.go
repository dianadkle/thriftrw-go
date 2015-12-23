// Copyright (c) 2015 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package compile

import "fmt"

// fakeScope is an implementation of Scope for testing. Instances may be
// constructed easily with the scope() function.
type fakeScope struct {
	types    map[string]TypeSpec
	services map[string]*Service
}

func (s fakeScope) LookupType(name string) (TypeSpec, error) {
	if t, ok := s.types[name]; ok {
		return t, nil
	}
	return nil, fmt.Errorf("unknown type: %s", name)
}

func (s fakeScope) LookupService(name string) (*Service, error) {
	if svc, ok := s.services[name]; ok {
		return svc, nil
	}
	return nil, fmt.Errorf("unknown service: %s", name)
}

// Helper to construct Scopes from the given pairs of items.
func scope(items ...struct {
	name  string
	value interface{}
}) Scope {
	scope := fakeScope{
		types:    make(map[string]TypeSpec),
		services: make(map[string]*Service),
	}

	for _, item := range items {
		name := item.name
		value := item.value
		switch v := value.(type) {
		case TypeSpec:
			scope.types[name] = v
		case *Service:
			scope.services[name] = v
		default:
			panic(fmt.Sprintf("unknown type %T of value %v", value, value))
		}
	}
	return scope
}