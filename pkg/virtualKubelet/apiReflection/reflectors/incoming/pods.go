package incoming

import (
	"context"
	ri "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors/reflectorsInterfaces"
	"github.com/liqotech/liqo/pkg/virtualKubelet/options"
	"github.com/liqotech/liqo/pkg/virtualKubelet/translation"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"
	"strings"
)

type PodsIncomingReflector struct {
	ri.APIReflector

	RemoteRemappedPodCIDR options.ReadOnlyOption
}

func (r *PodsIncomingReflector) SetSpecializedPreProcessingHandlers() {
	r.SetPreProcessingHandlers(ri.PreProcessingHandlers{
		AddFunc:    r.PreAdd,
		UpdateFunc: r.PreUpdate,
		DeleteFunc: r.PreDelete})
}

func (r *PodsIncomingReflector) HandleEvent(e interface{}) {
	event := e.(watch.Event)
	pod, ok := event.Object.(*corev1.Pod)
	if !ok {
		klog.Error("INCOMING REFLECTION: cannot cast object to pod")
		return
	}

	klog.V(3).Infof("INCOMING REFLECTION: received %v for pod %v", event.Type, pod.Name)

	r.PushToInforming(pod)
}

func (r *PodsIncomingReflector) PreAdd(obj interface{}) interface{} {
	return r.forgeTranslatedPod(obj)
}

func (r *PodsIncomingReflector) PreUpdate(newObj, _ interface{}) interface{} {
	return r.forgeTranslatedPod(newObj)
}

func (r *PodsIncomingReflector) PreDelete(obj interface{}) interface{} {
	return r.forgeTranslatedPod(obj)
}

func (r *PodsIncomingReflector) GetMirroredObject(namespace, name string) interface{} {
	informer := r.ForeignInformer(namespace)
	if informer == nil {
		return r.GetPodFromServer(namespace, name)
	}

	key := r.Keyer(namespace, name)
	obj, err := r.GetObjFromForeignCache(namespace, key)
	if err != nil {
		err = errors.Wrapf(err, "pod %v", key)
		klog.Error(err)
		return nil
	}

	return obj.(*corev1.Pod).DeepCopy()
}

func (r *PodsIncomingReflector) GetPodFromServer(namespace, name string) interface{} {
	pod, err := r.GetForeignClient().CoreV1().Pods(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		if !kerrors.IsNotFound(err) {
			klog.Error(err)
		}
		return nil
	}
	return pod
}

func (r *PodsIncomingReflector) KeyerFromObj(obj interface{}, remoteNamespace string) string {
	cm, ok := obj.(*corev1.Pod)
	if !ok {
		return ""
	}
	return strings.Join([]string{remoteNamespace, cm.Name}, "/")
}

func (r *PodsIncomingReflector) ListMirroredObjects(namespace string) []interface{} {
	return r.ForeignInformer(namespace).GetStore().List()
}

func (r *PodsIncomingReflector) CleanupNamespace(namespace string) {
	foreignNamespace, err := r.NattingTable().NatNamespace(namespace, false)
	if err != nil {
		klog.Error(err)
		return
	}

	// resync for ensuring to be remotely aligned with the foreign cluster state
	err = r.ForeignInformer(foreignNamespace).GetStore().Resync()
	if err != nil {
		klog.Errorf("error while resyncing pods foreign cache - ERR: %v", err)
		return
	}

	objects := r.ForeignInformer(foreignNamespace).GetStore().List()

	retriable := func(err error) bool {
		switch kerrors.ReasonForError(err) {
		case metav1.StatusReasonNotFound:
			return false
		default:
			klog.Warningf("retrying while deleting pod because of- ERR; %v", err)
			return true
		}
	}
	for _, obj := range objects {
		pod := r.forgeTranslatedPod(obj)
		if err := retry.OnError(retry.DefaultBackoff, retriable, func() error {
			return r.GetHomeClient().CoreV1().Pods(pod.Namespace).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})
		}); err != nil {
			klog.Errorf("Error while deleting remote pod %v/%v - ERR: %v", pod.Namespace, pod.Name, err)
		}
	}
}

func (r *PodsIncomingReflector) forgeTranslatedPod(obj interface{}) *corev1.Pod {
	po := obj.(*corev1.Pod).DeepCopy()
	nattedNs, err := r.NattingTable().DeNatNamespace(po.Namespace)
	if err != nil {
		klog.Error(err)
		return nil
	}

	return translation.F2HTranslate(po, r.RemoteRemappedPodCIDR.Value().ToString(), nattedNs)
}

func AddPodsIndexers() cache.Indexers {
	i := cache.Indexers{}
	i["pods"] = func(obj interface{}) ([]string, error) {
		po, ok := obj.(*corev1.Pod)
		if !ok {
			return []string{}, errors.New("cannot convert obj to pod")
		}
		return []string{
			strings.Join([]string{po.Namespace, po.Name}, "/"),
			po.Name,
		}, nil
	}
	return i
}
