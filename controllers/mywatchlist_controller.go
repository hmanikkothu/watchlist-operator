/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"net"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/go-logr/logr"
	"github.com/prometheus/common/log"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	webappv1 "my-watchlist.io/api/v1"
)

// MyWatchlistReconciler reconciles a MyWatchlist object
type MyWatchlistReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=webapp.demo.my-watchlist.io,resources=mywatchlists,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=webapp.demo.my-watchlist.io,resources=mywatchlists/status,verbs=get;update;patch

// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=list;watch;get;patch;create;update
// +kubebuilder:rbac:groups=core,resources=services,verbs=list;watch;get;patch;create;update

// Reconcile reconciles the request
func (r *MyWatchlistReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("mywatchlist", req.NamespacedName)

	log.Info("reconciling mywatchlist")

	var watchlist webappv1.MyWatchlist
	if err := r.Get(ctx, req.NamespacedName, &watchlist); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var redis webappv1.Redis
	redisName := client.ObjectKey{Name: watchlist.Spec.RedisName, Namespace: req.Namespace}
	if err := r.Get(ctx, redisName, &redis); err != nil {
		log.Error(err, "didn't get redis")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("got redis", "redis", redis.Name)

	deployment, err := r.createDeployment(watchlist, redis)
	if err != nil {
		return ctrl.Result{}, err
	}
	svc, err := r.createService(watchlist)
	if err != nil {
		return ctrl.Result{}, err
	}

	applyOpts := []client.PatchOption{client.ForceOwnership, client.FieldOwner("watchlist-controller")}

	err = r.Patch(ctx, &deployment, client.Apply, applyOpts...)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.Patch(ctx, &svc, client.Apply, applyOpts...)
	if err != nil {
		return ctrl.Result{}, err
	}

	watchlist.Status.URL = getServiceURL(svc, watchlist.Spec.Frontend.ServingPort)

	err = r.Status().Update(ctx, &watchlist)
	if err != nil {
		return ctrl.Result{}, err
	}

	log.Info("reconciled watchlist")

	return ctrl.Result{}, nil
}

func getServiceURL(svc corev1.Service, port int32) string {
	if len(svc.Status.LoadBalancer.Ingress) == 0 {
		log.Info("urlForService: LoadBalancer.Ingess not set")
		return ""
	}

	host := svc.Status.LoadBalancer.Ingress[0].Hostname
	if host == "" {
		host = svc.Status.LoadBalancer.Ingress[0].IP
	}

	return fmt.Sprintf("http://%s", net.JoinHostPort(host, fmt.Sprintf("%v", port)))
}

func (r *MyWatchlistReconciler) createService(watchlist webappv1.MyWatchlist) (corev1.Service, error) {
	svc := corev1.Service{
		TypeMeta: metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "Service"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      watchlist.Name,
			Namespace: watchlist.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{Name: "http", Port: 8080, Protocol: "TCP", TargetPort: intstr.FromString("http")},
			},
			Selector: map[string]string{"watchlist": watchlist.Name},
			Type:     corev1.ServiceTypeLoadBalancer,
		},
	}

	// set the controller reference to be able to cleanup during delete/gc
	if err := ctrl.SetControllerReference(&watchlist, &svc, r.Scheme); err != nil {
		return svc, err
	}

	return svc, nil
}

func (r *MyWatchlistReconciler) createDeployment(watchlist webappv1.MyWatchlist, redis webappv1.Redis) (appsv1.Deployment, error) {
	log.Info("HK - WR-createDeployment :###: RedisServiceName=" + redis.Status.RedisServiceName + "\n")

	depl := appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{APIVersion: appsv1.SchemeGroupVersion.String(), Kind: "Deployment"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      watchlist.Name,
			Namespace: watchlist.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: watchlist.Spec.Frontend.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"watchlist": watchlist.Name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"watchlist": watchlist.Name},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "frontend",
							Image: "hmanikkothu/watchlist:v1",
							Env: []corev1.EnvVar{
								{Name: "REDIS_HOST", Value: redis.Status.RedisServiceName},
							},
							Ports: []corev1.ContainerPort{
								{ContainerPort: 8080, Name: "http", Protocol: "TCP"},
							},
							Resources: *watchlist.Spec.Frontend.Resources.DeepCopy(),
						},
					},
				},
			},
		},
	}

	if err := ctrl.SetControllerReference(&watchlist, &depl, r.Scheme); err != nil {
		return depl, err
	}

	return depl, nil
}

func (r *MyWatchlistReconciler) watchlisAppUsingRedis(obj handler.MapObject) []ctrl.Request {
	listOptions := []client.ListOption{
		// matching our index
		client.MatchingField(".spec.redisName", obj.Meta.GetName()),
		// in the right namespace
		client.InNamespace(obj.Meta.GetNamespace()),
	}
	var list webappv1.MyWatchlistList
	if err := r.List(context.Background(), &list, listOptions...); err != nil {
		log.Error("watchlisAppUsingRedis: ", err)
		return nil
	}
	res := make([]ctrl.Request, len(list.Items))
	for i, watchlist := range list.Items {
		res[i].Name = watchlist.Name
		res[i].Namespace = watchlist.Namespace
	}
	return res
}

// SetupWithManager inits the controller
func (r *MyWatchlistReconciler) SetupWithManager(mgr ctrl.Manager) error {
	mgr.GetFieldIndexer().IndexField(
		&webappv1.MyWatchlist{}, ".spec.redisName",
		func(obj runtime.Object) []string {
			redisName := obj.(*webappv1.MyWatchlist).Spec.RedisName
			if redisName == "" {
				return nil
			}
			return []string{redisName}
		})

	return ctrl.NewControllerManagedBy(mgr).
		For(&webappv1.MyWatchlist{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.Deployment{}).
		Watches(
			&source.Kind{Type: &webappv1.Redis{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: handler.ToRequestsFunc(r.watchlisAppUsingRedis),
			}).
		Complete(r)
}
