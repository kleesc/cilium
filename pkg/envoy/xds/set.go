// Copyright 2018 Authors of Cilium
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package xds

import (
	"context"

	"github.com/cilium/cilium/pkg/envoy/api"
	"github.com/cilium/cilium/pkg/lock"

	"github.com/golang/protobuf/proto"
)

// ResourceSet is a versioned set of resources.
// A set associates a single version to all the resources it contains.
// The version is monotonically increased for any change to the set.
type ResourceSet interface {
	// GetResources returns the current version of the resources with the given
	// names.
	// If lastVersion is not nil and the resources with the given names haven't
	// changed since lastVersion, nil is returned.
	// If resourceNames is empty, all resources are returned.
	// Should not be blocking.
	GetResources(ctx context.Context, typeURL string, lastVersion *uint64,
		node *api.Node, resourceNames []string) (*VersionedResources, error)
}

// VersionedResources is a set of protobuf-encoded resources along with their
// version.
type VersionedResources struct {
	// Version is the version of the resources.
	Version uint64

	// Resources is the set of protobuf-encoded resources.
	// May be empty.
	Resources []proto.Message

	// Canary indicates whether the client should only do a dry run of
	// using  the resources.
	Canary bool
}

// ObservableResourceSet is a ResourceSet that allows registering observers of
// new resource versions from this set.
type ObservableResourceSet interface {
	ResourceSet

	// AddResourceVersionObserver registers an observer of new versions of
	// resources from this set.
	AddResourceVersionObserver(listener ResourceVersionObserver)

	// RemoveResourceVersionObserver unregisters an observer of new versions of
	// resources from this set.
	RemoveResourceVersionObserver(listener ResourceVersionObserver)
}

// ResourceVersionObserver defines the HandleNewResourceVersion method which is
// called whenever the version of the resources of a given type has changed.
type ResourceVersionObserver interface {
	// HandleNewResourceVersion notifies of a new version of the resources of
	// the given type.
	HandleNewResourceVersion(typeURL string, version uint64)
}

// BaseObservableResourceSet implements the AddResourceVersionObserver and
// RemoveResourceVersionObserver methods to handle the notification of new
// resource versions. This is meant to be used as a base to implement
// ObservableResourceSet.
type BaseObservableResourceSet struct {
	// locker is the locker used to synchronize all accesses to this set.
	locker lock.RWMutex

	// observers is the set of registered observers.
	observers map[ResourceVersionObserver]struct{}
}

// NewBaseObservableResourceSet initializes the given set.
func NewBaseObservableResourceSet() *BaseObservableResourceSet {
	return &BaseObservableResourceSet{
		observers: make(map[ResourceVersionObserver]struct{}),
	}
}

// AddResourceVersionObserver registers an observer to be notified of new
// resource version.
func (s *BaseObservableResourceSet) AddResourceVersionObserver(observer ResourceVersionObserver) {
	s.locker.Lock()
	defer s.locker.Unlock()

	s.observers[observer] = struct{}{}
}

// RemoveResourceVersionObserver unregisters an observer that was previously
// registered by calling AddResourceVersionObserver.
func (s *BaseObservableResourceSet) RemoveResourceVersionObserver(observer ResourceVersionObserver) {
	s.locker.Lock()
	defer s.locker.Unlock()

	delete(s.observers, observer)
}

// NotifyNewResourceVersionRLocked notifies registered observers that a new version of
// the resources of the given type is available.
// This function MUST be called with locker's lock acquired.
func (s *BaseObservableResourceSet) NotifyNewResourceVersionRLocked(typeURL string, version uint64) {
	for o := range s.observers {
		o.HandleNewResourceVersion(typeURL, version)
	}
}
