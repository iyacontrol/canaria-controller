package canaria

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
	log "k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/iyacontrol/canaria-controller/pkg/apis"
	canariav1beta1 "github.com/iyacontrol/canaria-controller/pkg/apis/canaria/v1beta1"
)

var replicas int32 = 1

var sche = runtime.NewScheme()

// Add creates a new cronhpa Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	clientConfig := mgr.GetConfig()

	clientSet, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		log.Fatal(err)
	}

	options := ctrl.Options{Scheme: sche}
	canariaClientSet, err := client.New(clientConfig, client.Options{Scheme: options.Scheme})
	if err != nil {
		log.Fatal(err)
	}

	evtNamespacer := clientSet.CoreV1()
	broadcaster := record.NewBroadcaster()
	broadcaster.StartLogging(log.Infof)
	broadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: evtNamespacer.Events("")})
	recorder := broadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "canaria-deploy-controller"})

	return &ReconcileCanaria{
		Client:           mgr.GetClient(),
		scheme:           mgr.GetScheme(),
		clientSet:        clientSet,
		canariaClientSet: canariaClientSet,
		eventRecorder:    recorder,
	}
}

var _ reconcile.Reconciler = &ReconcileCanaria{}

// ReconcileCanaria reconciles a Canaria object
type ReconcileCanaria struct {
	client.Client
	scheme        *runtime.Scheme
	syncPeriod    time.Duration
	eventRecorder record.EventRecorder

	clientSet        kubernetes.Interface
	canariaClientSet client.Client
}

// When the CHPA is changed (status is changed, edited by the user, etc),
// a new "UpdateEvent" is generated and passed to the "updatePredicate" function.
// If the function returns "true", the event is added to the "Reconcile" queue,
// If the function returns "false", the event is skipped.
func updatePredicate(ev event.UpdateEvent) bool {
	oldObject := ev.ObjectOld.(*canariav1beta1.Canaria)
	newObject := ev.ObjectNew.(*canariav1beta1.Canaria)
	// Add the chpa object to the queue only if the spec has changed.
	// Status change should not lead to a requeue.
	if !apiequality.Semantic.DeepEqual(newObject.Spec, oldObject.Spec) {
		return true
	}
	return false
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("canaria-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to CHPA
	predicate := predicate.Funcs{UpdateFunc: updatePredicate}
	err = c.Watch(&source.Kind{Type: &canariav1beta1.Canaria{}}, &handler.EnqueueRequestForObject{}, predicate)
	if err != nil {
		return err
	}

	return nil
}

// Reconcile reads that state of the cluster for a Canaria object and makes changes based on the state read
// and what is in the Canaria.Spec
// The implementation repeats kubernetes hpa implementation from v1.10.8

// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// TODO: decide, what to use: patch or update in rbac
// +kubebuilder:rbac:groups=canaria.shareit.me,resources=canarias,verbs=get;list;watch;create;update;patch;delete
func (r *ReconcileCanaria) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.Infof("Reconcile request: %v\n", request)

	canaria := &canariav1beta1.Canaria{}

	err := r.Get(context.TODO(), request.NamespacedName, canaria)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}

		log.Errorf("Faild get Canaria: %s", err.Error())
		return reconcile.Result{}, err
	}

	canariaStatusOriginal := canaria.Status.DeepCopy()

	var deploy appsv1.Deployment
	err = r.Get(context.TODO(), request.NamespacedName, &deploy)
	if err != nil {
		return reconcile.Result{}, err
	}

	canariaDeployName := request.Name + "-canary"

	canary := appsv1.Deployment{
		TypeMeta: deploy.TypeMeta,
		ObjectMeta: metav1.ObjectMeta{
			Name:        canariaDeployName,
			Namespace:   request.Namespace,
			Labels:      deploy.Labels,
			Annotations: deploy.Annotations,
		},
		Spec: deploy.Spec,
	}

	for containerName, image := range canaria.Spec.Images {
		for i, c := range deploy.Spec.Template.Spec.Containers {
			if c.Name == containerName {
				deploy.Spec.Template.Spec.Containers[i].Image = image
			}
		}
	}

	if canaria.Spec.TargetSize > 0 {
		canary.Spec.Replicas = &canaria.Spec.TargetSize
	} else {
		canary.Spec.Replicas = &replicas
	}

	canariaExist := true
	var oldCanaria appsv1.Deployment
	if err := r.Get(context.TODO(), types.NamespacedName{
		Namespace: request.Namespace,
		Name:      canariaDeployName,
	}, &oldCanaria); err != nil {
		if !errors.IsNotFound(err) {
			log.Errorf("err can not find canaria deployment: %s in %s", canariaDeployName, request.Namespace)
			return reconcile.Result{}, err
		}
		canariaExist = false
	}

	switch canaria.Spec.Stage {
	case canariav1beta1.K8sDeployStageCanary:
		if canariaExist {
			if err := r.Update(context.TODO(), &canary); err != nil {
				log.Errorf("unable to update canaria deployment of %s in %s: %v", canariaDeployName, request.Namespace, err)
				r.eventRecorder.Event(canaria, v1.EventTypeWarning, "FailedUpdateCanaria", err.Error())
				return ctrl.Result{}, err
			}
			log.Infof("update canaria deployment name: %s in %s", canariaDeployName, request.Namespace)

		} else {
			if err := r.Create(context.TODO(), &canary); err != nil {
				log.Errorf("unable to create canary deployment of %s in %s: %v", request.Name, request.Namespace, err)
				r.eventRecorder.Event(canaria, v1.EventTypeWarning, "FailedCreateCanaria", err.Error())
				return reconcile.Result{}, err
			}

			log.Infof("create canary deployment name: %s in %s", canariaDeployName, request.Namespace)
		}
		r.setStatus(canaria)
		r.updateStatusIfNeeded(canariaStatusOriginal, canaria)

	case canariav1beta1.K8sDeployStageRollBack:
		if canariaExist {
			if err := r.Delete(context.TODO(), &oldCanaria); err != nil {
				log.Errorf("unable to delete canaria deployment of %s in %s: %v", canariaDeployName, request.Namespace, err)
				r.eventRecorder.Event(canaria, v1.EventTypeWarning, "FailedDeleteCanaria", err.Error())
				return reconcile.Result{}, err
			}
			log.Infof("delete canaria deployment name: %s in %s", canariaDeployName, request.Namespace)

			r.setStatus(canaria)
			r.updateStatusIfNeeded(canariaStatusOriginal, canaria)
		} else {
			log.Infof("canaria deployment name: %s in %s not exist, skip delete action", canariaDeployName, request.Namespace)
		}

	case canariav1beta1.K8sDeployStageRollup:
		if canariaExist {
			if err := r.Delete(context.TODO(), &oldCanaria); err != nil {
				log.Errorf("unable to delete canaria deployment of %s in %s: %v", canariaDeployName, request.Namespace, err)
				r.eventRecorder.Event(canaria, v1.EventTypeWarning, "FailedDeleteCanaria", err.Error())
				return reconcile.Result{}, err
			}

			log.Infof("delete canaria deployment name: %s in %s", canariaDeployName, request.Namespace)
		} else {
			log.Infof("canaria deployment name: %s in %s not exist, skip delete action", canariaDeployName, request.Namespace)
		}

		for containerName, image := range canaria.Spec.Images {
			for i, c := range deploy.Spec.Template.Spec.Containers {
				if c.Name == containerName {
					deploy.Spec.Template.Spec.Containers[i].Image = image
				}
			}
		}

		if err := r.Update(context.TODO(), &deploy); err != nil {
			log.Errorf("unable to update deployment of %s in %s: %v", canariaDeployName, request.Namespace, err)
			r.eventRecorder.Event(canaria, v1.EventTypeWarning, "FailedUpdateCanaria", err.Error())
			return reconcile.Result{}, err
		}

		r.setStatus(canaria)
		r.updateStatusIfNeeded(canariaStatusOriginal, canaria)

		log.Infof("update container images : %v of  deployment name: %s in %s", canaria.Spec.Images, request.Name, request.Namespace)
	}

	return ctrl.Result{}, nil
}

func (r *ReconcileCanaria) updateStatusIfNeeded(oldStatus *canariav1beta1.CanariaStatus, newCanaria *canariav1beta1.Canaria) error {
	// skip a write if we wouldn't need to update
	if apiequality.Semantic.DeepEqual(oldStatus, newCanaria.Status) {
		return nil
	}
	return r.updateCanaria(newCanaria)
}

func (r *ReconcileCanaria) updateCanaria(canaria *canariav1beta1.Canaria) error {
	return r.Update(context.TODO(), canaria)
}

func (r *ReconcileCanaria) setStatus(canaria *canariav1beta1.Canaria) {
	now := metav1.NewTime(time.Now())
	canaria.Status.LastUpdateTime = &now
}

func init() {
	apis.AddToScheme(sche)
}
