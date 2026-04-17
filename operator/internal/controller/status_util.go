package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// UpdateStatusWithRetry safely updates the status of a Kubernetes object, retrying on conflict.
// It fetches the latest version of the object from the API server before applying the status changes.
func UpdateStatusWithRetry[T client.Object](ctx context.Context, c client.Client, req types.NamespacedName, originalObj T, applyStatus func(T)) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latestObj := originalObj.DeepCopyObject().(T)
		if err := c.Get(ctx, req, latestObj); err != nil {
			return err
		}
		applyStatus(latestObj)
		// We explicitly do NOT use Patch, because Status updates often overwrite entire structures
		return c.Status().Update(ctx, latestObj)
	})
}
