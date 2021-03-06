// Copyright © 2017 The virtual-kubelet authors
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

package node

import (
	"context"
	"k8s.io/klog"

	"github.com/liqotech/liqo/internal/utils/trace"
	pkgerrors "github.com/pkg/errors"
	"k8s.io/client-go/util/workqueue"
)

const (
	// maxRetries is the number of times we try to process a given key before permanently forgetting it.
	maxRetries = 20
)

type queueHandler func(ctx context.Context, key string) error

func handleQueueItem(ctx context.Context, q workqueue.RateLimitingInterface, handler queueHandler) bool {
	ctx, span := trace.StartSpan(ctx, "handleQueueItem")
	defer span.End()

	obj, shutdown := q.Get()
	if shutdown {
		return false
	}

	klog.V(5).Info("Got queue object")

	err := func(obj interface{}) error {
		defer klog.V(5).Info("Processed queue item")
		// We call Done here so the work queue knows we have finished processing this item.
		// We also must remember to call Forget if we do not want this work item being re-queued.
		// For example, we do not call Forget if a transient error occurs.
		// Instead, the item is put back on the work queue and attempted again after a back-off period.
		defer q.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the work queue.
		// These are of the form namespace/name.
		// We do this as the delayed nature of the work queue means the items in the informer cache may actually be more up to date that when the item was initially put onto the workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the work queue is actually invalid, we call Forget here else we'd go into a loop of attempting to process a work item that is invalid.
			q.Forget(obj)
			klog.Warningf("expected string in work queue item but got %#v", obj)
			return nil
		}

		// Add the current key as an attribute to the current span.
		ctx = span.WithField(ctx, "key", key)
		// Run the syncHandler, passing it the namespace/name string of the Pod resource to be synced.
		if err := handler(ctx, key); err != nil {
			if q.NumRequeues(key) < maxRetries {
				// Put the item back on the work queue to handle any transient errors.
				klog.Warningf("requeuing %q due to failed sync, error: %s", key, err.Error())
				q.AddRateLimited(key)
				return nil
			}
			// We've exceeded the maximum retries, so we must forget the key.
			q.Forget(key)
			return pkgerrors.Wrapf(err, "forgetting %q due to maximum retries reached", key)
		}
		// Finally, if no error occurs we Forget this item so it does not get queued again until another change happens.
		q.Forget(obj)
		return nil
	}(obj)

	if err != nil {
		// We've actually hit an error, so we set the span's status based on the error.
		span.SetStatus(err)
		klog.Error(err)
		return true
	}

	return true
}

func (pc *PodController) runSyncPodStatusFromProviderWorker(ctx context.Context, workerID string, q workqueue.RateLimitingInterface) {
	for pc.processPodStatusUpdate(ctx, workerID, q) {
	}
}

func (pc *PodController) processPodStatusUpdate(ctx context.Context, workerID string, q workqueue.RateLimitingInterface) bool {
	ctx, span := trace.StartSpan(ctx, "processPodStatusUpdate")
	defer span.End()

	// Add the ID of the current worker as an attribute to the current span.
	ctx = span.WithField(ctx, "workerID", workerID)

	return handleQueueItem(ctx, q, pc.podStatusHandler)
}
