package auth_service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog"
	"strings"
)

func (authService *AuthServiceCtrl) getToken() (string, error) {
	obj, exists, err := authService.secretInformer.GetStore().GetByKey(strings.Join([]string{authService.namespace, "auth-token"}, "/"))
	if err != nil {
		klog.Error(err)
		return "", err
	} else if !exists {
		err = kerrors.NewNotFound(schema.GroupResource{
			Group:    "v1",
			Resource: "secrets",
		}, "auth-token")
		klog.Error(err)
		return "", err
	}

	secret, ok := obj.(*v1.Secret)
	if !ok {
		err = kerrors.NewNotFound(schema.GroupResource{
			Group:    "v1",
			Resource: "secrets",
		}, "auth-token")
		klog.Error(err)
		return "", err
	}

	return authService.getTokenFromSecret(secret.DeepCopy())
}

func (authService *AuthServiceCtrl) createToken() error {
	_, exists, _ := authService.secretInformer.GetStore().GetByKey(strings.Join([]string{authService.namespace, "auth-token"}, "/"))
	if !exists {
		token, err := generateToken()
		if err != nil {
			return err
		}

		secret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: "auth-token",
			},
			StringData: map[string]string{
				"token": token,
			},
		}
		_, err = authService.clientset.CoreV1().Secrets(authService.namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
		if err != nil && !kerrors.IsAlreadyExists(err) {
			klog.Error(err)
			return err
		}
	}
	return nil
}

func (authService *AuthServiceCtrl) getTokenFromSecret(secret *v1.Secret) (string, error) {
	v, ok := secret.Data["token"]
	if !ok {
		// TODO: specialise secret type
		err := errors.New("invalid secret")
		klog.Error(err)
		return "", err
	}
	return string(v), nil
}

func generateToken() (string, error) {
	b := make([]byte, 64)
	_, err := rand.Read(b)
	if err != nil {
		klog.Error(err)
		return "", err
	}
	return fmt.Sprintf("%x", b), nil
}
