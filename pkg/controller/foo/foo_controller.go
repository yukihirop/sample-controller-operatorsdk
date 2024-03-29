package foo

import (
	"context"

	samplecontrollerv1alpha1 "github.com/yukihirop/sample-controller-operatorsdk/pkg/apis/samplecontroller/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_foo")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Foo Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileFoo{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("foo-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Foo
	err = c.Watch(&source.Kind{Type: &samplecontrollerv1alpha1.Foo{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Deployment and requeue the owner Foo
	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &samplecontrollerv1alpha1.Foo{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileFoo implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileFoo{}

// ReconcileFoo reconciles a Foo object
type ReconcileFoo struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Foo object and makes changes based on the state read
// and what is in the Foo.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileFoo) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Foo")
	ctx := context.Background()

	/*
		#### 1: Load the Foo by name
		We'll fetch the Foo using our client.
		All client methods take a context (to allow for cancellation) as
		their first argument, and the object
		in question as their last.
		Get is a bit special, in that it takes a
		[`NamespacedName`](https://godoc.org/sigs.k8s.io/controller-runtime/pkg/client#ObjectKey)
		as the middle argument (most don't have a middle argument, as we'll see below).

		Many client methods also take variadic options at the end.
	*/
	foo := &samplecontrollerv1alpha1.Foo{}
	if err := r.client.Get(ctx, request.NamespacedName, foo); err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. FOr additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Info("Foo not found. Ignore not found")
			return reconcile.Result{}, nil
		}
		reqLogger.Error(err, "failed to get Foo")
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	/*
		#### 2: Clean Up old Deployment which had been owned by Foo Resource.
		We'll find deployment object which foo object owns.
		If there is a deployment which is owned by foo and it doesn't match foo.spec.deploymentName,
		we clean up the deployment object.
		(If we do nothing without this func, the old deployment object keeps existing.)
	*/
	if err := r.cleanupOwnedResources(ctx, foo); err != nil {
		reqLogger.Error(err, "failed to clean up old Deployment resources for this Foo")
		return reconcile.Result{}, err
	}

	/*
		#### 3: Create deployment if foo  doesn't have it.
		We will get deployment which have foo.spec.deploymentName.
		If it doesn't exist, we'll use Create method.
	*/

	// get deploymentName from foo.Spec
	deploymentName := foo.Spec.DeploymentName

	// define deployment template using deploymentName
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: request.Namespace,
		},
	}

	// create deployment which has deploymentName and replicas by newDeployment method if it doesn't exist
	if err := r.client.Get(ctx, client.ObjectKey{Namespace: foo.Namespace, Name: deploymentName}, deployment); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("could not find existing Deployment for Foo, creating one...")
			// create deployment template using newDeployment method
			deployment = newDeployment(foo)

			// create deployment object
			if err := r.client.Create(ctx, deployment); err != nil {
				reqLogger.Error(err, "failed to create Deployment resource")
				// Error creating the object - requeue the request.
				return reconcile.Result{}, err
			}

			reqLogger.Info("created Deployment resource for Foo")
			return reconcile.Result{}, nil
		}

		reqLogger.Error(err, "failed to get Deployment for Foo resource")
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	/*
		### 4: Update deployment spec if it isn't desire state.
		We compare foo.spec.replicas and deployment.spec.replicas.
		If it doesn't correct, we'll use Update method to reconcile deployment state.
	*/

	// compare foo.spec.replicas and deployment.spec.replicas
	if foo.Spec.Replicas != nil && *foo.Spec.Replicas != *deployment.Spec.Replicas {
		reqLogger.Info("unmatch spec","foo.spec.replicas", foo.Spec.Replicas, "deployment.spec.replicas", deployment.Spec.Replicas)
		reqLogger.Info("Deployment replicas is not equal Foo replicas. reconcile this...")
		// Update deployment spec
		if err := r.client.Update(ctx, newDeployment(foo)); err != nil {
			reqLogger.Error(err, "failed to update Deployment for Foo resource")
			// Error updating the object - requeue the request.
			return reconcile.Result{}, err
		}

		reqLogger.Info("updated Deployment spec for Foo")
		return reconcile.Result{}, nil
	}

	/*
		#### 5: Update foo status.
		if foo.Status.AvailableReplicas doesn't match deployment.Status.AvailableReplicas,
		we need to update foo.Status.AvailableReplicas.
		we will use Update method to reconcile foo state.
	*/

	// compare foo.status.availableReplicas and deployment.status.availableReplicas
	if foo.Status.AvailableReplicas != deployment.Status.AvailableReplicas {
		reqLogger.Info("updating Foo status")
		foo.Status.AvailableReplicas = deployment.Status.AvailableReplicas
		// Update foo spec
		if err := r.client.Update(ctx, foo); err != nil {
			reqLogger.Error(err, "failed to update Foo status")
			// Error updating the object - requeue the request.
			return reconcile.Result{}, err
		}

		reqLogger.Info("updated Foo status", "foo.status.availableReplicas", foo.Status.AvailableReplicas)
	}

	return reconcile.Result{}, nil
}

// cleanupOwnedResources will Delete any existing Deployment resources that
// were created for the given Foo that no longer match the
// foo.spec.deploymentName field.
func (r *ReconcileFoo) cleanupOwnedResources(ctx context.Context, foo *samplecontrollerv1alpha1.Foo) error {
	reqLogger := log.WithValues("Request.Namespace", foo.Namespace, "Request.Name", foo.Name)
	reqLogger.Info("finding existing Deployments for Foo resource")

	// List all deployment resources owned by this Foo
	deployments := &appsv1.DeploymentList{}
	labelSelector := labels.SelectorFromSet(labelsForFoo(foo.Name))
	listOps := &client.ListOptions{
		Namespace:     foo.Namespace,
		LabelSelector: labelSelector,
	}
	if err := r.client.List(ctx, listOps, deployments); err != nil {
		reqLogger.Error(err, "failed to get list of deployments")
		return err
	}

	// Delete deployment if the deployment name does't match foo.spec.deploymentName
	for _, deployment := range deployments.Items {
		if deployment.Name == foo.Spec.DeploymentName {
			// If this deployment's name matches the one on the Foo resource
			// then do not delete it.
			continue
		}

		// Delete old deployment object which doesn't match foo.spec.deploymentName
		if err := r.client.Delete(ctx, &deployment); err != nil {
			reqLogger.Error(err, "failed to delete Deployment resource")
			return err
		}

		reqLogger.Info("deleted old Deployment resource for Foo", "deploymentName", deployment.Name)
	}

	return nil
}

// labelsForFoo returns the labels for selecting the resources
// belonging to the given foo CR name.
func labelsForFoo(name string) map[string]string {
	return map[string]string{"app": "nginx", "controller": name}
}

// newDeployment creates a new Deployment for a Foo resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the Foo resource that 'owns' it.
func newDeployment(foo *samplecontrollerv1alpha1.Foo) *appsv1.Deployment {
	labels := map[string]string{
		"app":        "nginx",
		"controller": foo.Name,
	}
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      foo.Spec.DeploymentName,
			Namespace: foo.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(foo, samplecontrollerv1alpha1.SchemeGroupVersion.WithKind("Foo")),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: foo.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "nginx",
						Image: "nginx:latest",
					},
				},
			},
		},
	}
}
