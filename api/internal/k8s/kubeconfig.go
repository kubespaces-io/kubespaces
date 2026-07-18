package k8s

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Kubeconfig reads the tenant kubeconfig from the Secret referenced by the
// Tenant CR's status.kubeconfigSecretRef, in the tenant's target namespace.
// Returns ErrNotFound if the CR is missing, ErrNotReady if the reference or
// secret data is not available yet.
func (c *Client) Kubeconfig(ctx context.Context, name string) ([]byte, error) {
	state, err := c.GetTenantState(ctx, name)
	if err != nil {
		return nil, err
	}
	ref := state.KubeconfigSecretRef
	if ref.Name == "" || ref.Key == "" {
		return nil, ErrNotReady
	}

	secret, err := c.core.CoreV1().Secrets(state.TargetNamespace).Get(ctx, ref.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil, ErrNotReady
	}
	if err != nil {
		return nil, fmt.Errorf("get kubeconfig secret: %w", err)
	}
	data, ok := secret.Data[ref.Key]
	if !ok || len(data) == 0 {
		return nil, ErrNotReady
	}
	return data, nil
}
